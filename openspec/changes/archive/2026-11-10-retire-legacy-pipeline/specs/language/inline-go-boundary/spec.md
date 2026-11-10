## REMOVED Requirements

### Requirement: The legacy backend rejects raw declarations explicitly
**Reason**: The `--legacy` pipeline is removed, so there is no separate backend that needs to reject `raw:`. The bootstrap backends already support inline Go with and without `raw:` under the remaining requirements.
**Migration**: Use the default bootstrap pipeline. No source change is required for blocks without `raw:`; blocks with `raw:` keep their bootstrap behavior.
