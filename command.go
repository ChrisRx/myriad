package distro

type Op int

const (
	Get Op = iota
	Set
	Delete
)

type Command struct {
	Op    Op     `json:"op,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}
