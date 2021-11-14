
# Eth-state-transition

TODO:
- Clean the executor/transition
- What do we do with the types?
- Snapshot using GetNode?
- Move some types (account) to root state.
- Separate trie (only inmemory?) from backends?
    - Two implementtions of State: archive and full.
    - Play with the itrie from there. But keep some generic functions if needed for outside development.
- Commit returns Snapshot? This only done for testing I think, the backend should optimize for that.
    - For example: bulk writting of several blocks and only check commit once (not a bad idea).
- Circle dependency with runtime and EVM...
