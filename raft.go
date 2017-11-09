package distro

import (
	"encoding/json"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"time"

	"github.com/ChrisRx/jolt"
	"github.com/ChrisRx/raft-badger"
	"github.com/dgraph-io/badger"
	"github.com/google/uuid"
	"github.com/hashicorp/raft"
	"github.com/pkg/errors"
)

type Raft struct {
	*raft.Raft

	RPC *RaftRPC

	id     uuid.UUID
	config *Config
	fsm    *Store

	logger *jolt.Logger
}

func NewRaft(c *Config) (r *Raft, err error) {
	if c.Logger == nil {
		c.Logger = jolt.DefaultLogger()
	}
	store := &Store{
		kv: make(map[string]string),
	}
	r = &Raft{
		config: c,
		id:     uuid.New(),
		logger: c.Logger,
		fsm:    store,
	}
	r.RPC = &RaftRPC{r}
	c.Store = store
	transport := raft.NewNetworkTransport(c.StreamLayer, 3, 10*time.Second, os.Stderr)
	var db raftbadger.LogStableStore
	var snapshots raft.SnapshotStore
	if c.Dir == "" {
		db = raft.NewInmemStore()
		snapshots = raft.NewDiscardSnapshotStore()
	} else {
		c.Dir = filepath.Join(c.Dir, r.id.String())
		opt := badger.DefaultOptions
		opt.Dir = c.Dir
		path, err := filepath.Abs(c.Dir)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			os.MkdirAll(path, os.ModePerm)
		}
		opt.ValueDir = c.Dir
		db, err = raftbadger.New(opt)
		if err != nil {
			return nil, err
		}
		snapshots, err = raft.NewFileSnapshotStore(c.Dir, c.RetainSnapshotCount, os.Stderr)
		if err != nil {
			return nil, err
		}
	}
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(r.id.String())
	config.Logger = logger
	r.Raft, err = raft.NewRaft(config, r.fsm, db, db, snapshots, transport)
	if err != nil {
		return nil, err
	}
	if c.EnableSingle {
		r.Raft.BootstrapCluster(raft.Configuration{
			Servers: []raft.Server{
				raft.Server{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		})
	}
	return r, nil
}

func (r *Raft) Config() raft.Configuration {
	future := r.Raft.GetConfiguration()
	if err := future.Error(); err != nil {
		r.logger.Print(err)
		return future.Configuration()
	}
	return future.Configuration()
}

func (r *Raft) LocalID() string {
	return r.id.String()
}

func (r *Raft) IsLeader() bool {
	return r.Raft.State() == raft.Leader
}

func (r *Raft) Barrier() error {
	if !r.IsLeader() {
		return raft.ErrNotLeader
	}
	f := r.Raft.Barrier(r.config.RaftTimeout)
	if err := f.Error(); err != nil {
		return err
	}
	return nil
}

func (r *Raft) Join(id, addr string) error {
	r.logger.Print("received join request for remote node as %s", addr)
	for {
		f := r.Raft.AddVoter(raft.ServerID(id), raft.ServerAddress(addr), 0, 0)
		if f.Error() == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	r.logger.Print("node at %s joined successfully", addr)
	return nil
}

func (r *Raft) Get(key string) (string, error) {
	if !r.IsLeader() {
		var v string
		err := r.RPC.Get(&Command{Key: key}, &v)
		if err != nil {
			return "", err
		}
		return v, nil
	}
	if val, ok := r.fsm.Get(key); ok {
		return val, nil
	}
	return "", errors.Errorf("key not found: %#v", key)
}

func (r *Raft) Set(key, value string) error {
	if !r.IsLeader() {
		var resp string
		err := r.RPC.Set(&Command{Key: key, Value: value}, &resp)
		if err != nil {
			return err
		}
		return nil
	}
	b, err := json.Marshal(&Command{Set, key, value})
	if err != nil {
		return errors.WithStack(err)
	}
	f := r.Raft.Apply(b, r.config.RaftTimeout)
	return f.Error()
}

func (r *Raft) Delete(key string) error {
	if !r.IsLeader() {
		return raft.ErrNotLeader
		var resp string
		err := r.RPC.Delete(&Command{Key: key}, &resp)
		if err != nil {
			return err
		}
		return nil
	}
	b, err := json.Marshal(&Command{Delete, key, ""})
	if err != nil {
		return err
	}
	f := r.Raft.Apply(b, r.config.RaftTimeout)
	return f.Error()
}

type RaftRPC struct {
	raft *Raft
}

func (r *RaftRPC) newClient() (*rpc.Client, error) {
	conn, err := net.Dial("tcp", string(r.raft.Leader()))
	if err != nil {
		return nil, err
	}
	conn.Write([]byte{byte(rpcCommand)})
	client := rpc.NewClient(conn)
	return client, nil
}

func (r *RaftRPC) Get(c *Command, resp *string) error {
	if r.raft.IsLeader() {
		v, err := r.raft.Get(c.Key)
		if err != nil {
			return err
		}
		*resp = v
		return nil
	}
	client, err := r.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.Call("RaftRPC.Get", c, resp)
	if err != nil {
		return err
	}
	return nil
}

func (r *RaftRPC) Set(c *Command, resp *string) error {
	if r.raft.IsLeader() {
		if err := r.raft.Set(c.Key, c.Value); err != nil {
			return err
		}
		return nil
	}
	client, err := r.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.Call("RaftRPC.Set", c, resp)
	if err != nil {
		return err
	}
	return nil
}

func (r *RaftRPC) Delete(c *Command, resp *string) error {
	if r.raft.IsLeader() {
		if err := r.raft.Delete(c.Key); err != nil {
			return err
		}
		return nil
	}
	client, err := r.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.Call("RaftRPC.Delete", c, resp)
	if err != nil {
		return err
	}
	return nil
}
