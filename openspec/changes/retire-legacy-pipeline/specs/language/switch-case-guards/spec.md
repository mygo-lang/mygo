## REMOVED Requirements

### Requirement: Legacy pipeline rejects guard syntax explicitly
**Reason**: The `--legacy sync` pipeline is removed, so there is no second backend that could ignore or partially implement guards.
**Migration**: Use the default bootstrap pipeline, which accepts guarded cases under the remaining requirements. No source change is required.
