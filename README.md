# myriad

**At very early stages, not ready to be used yet.**

myriad is a library to aid in the creation of distributed systems with consensus and membership. The goal is to produce an easy to use package that will help develop distributed applications that are autonomous by maintaining nodes that fulfill the requirements for fault-tolerant consensus and then only maintain membership for subsequent nodes that join the cluster. In practice, I am hoping that keeping the raft cluster very small will keep re-election times relatively low, while allowing the same application to continue to scale-out and have access to all benefits provided by a consensus-based key/value store (via raft backed by badger) but not require it to be separately maintained from the application.
