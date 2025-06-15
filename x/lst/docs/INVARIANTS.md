# LST Module Invariant Checks

This document describes the invariant check system implemented for the LST (Liquid Staking Token) module. Invariants provide safety guarantees by detecting state inconsistencies that could indicate bugs or protocol violations.

## Overview

The LST module implements four comprehensive invariant checks that verify different aspects of the module's state consistency:

1. **Basket Accounting Invariant**: Verifies that basket token supply matches underlying stake
2. **Module Accounts Invariant**: Ensures module account balances are consistent with pending operations
3. **Pending Redemptions Invariant**: Validates pending redemption record consistency
4. **Basket State Invariant**: Checks internal basket state consistency

## Invariant Details

### 1. Basket Accounting Invariant (`basket-accounting`)

**Purpose**: Ensures that the total value backing basket tokens equals the claimed staked amount.

**Checks**:
- `actual_staked + pending_unbonding + module_balance = total_staked_tokens`
- No negative shares or staked amounts
- Exchange rates are within reasonable bounds (0.1 to 10.0)
- Validates delegation amounts match recorded values

**Formula**:
```
Total Accounted Value = Σ(validator_delegations) + Σ(pending_unbondings) + module_liquid_balance
Expected Value = basket.TotalStakedTokens
Tolerance = 1000 utia (for rounding errors)
```

**Violation Examples**:
- Slashing events not properly reflected
- Bugs in delegation/undelegation accounting
- Inflation/deflation attacks on basket value

### 2. Module Accounts Invariant (`module-accounts`)

**Purpose**: Ensures module account balances reflect pending operations correctly.

**Checks**:
- Basket module accounts don't hold excessive balances
- Main module account stays below reasonable limits (1 TIA)
- Module balances correspond to completed but unpaid redemptions

**Violation Examples**:
- Tokens stuck in module accounts
- Failed payout mechanisms
- Rewards not properly distributed

### 3. Pending Redemptions Invariant (`pending-redemptions`)

**Purpose**: Validates the consistency of pending redemption records.

**Checks**:
- All referenced baskets exist
- Redemption amounts are positive and reasonable
- Shares burned ≤ basket total shares
- Delegator addresses are valid
- Completion times are not excessively old

**Violation Examples**:
- Orphaned redemption records
- Invalid redemption amounts
- Address corruption

### 4. Basket State Invariant (`basket-state`)

**Purpose**: Checks internal consistency of basket definitions.

**Checks**:
- No duplicate basket IDs or denoms
- Validator weights sum to 1.0 (within tolerance)
- No duplicate validators within baskets
- Valid validator and creator addresses
- Proper basket metadata

**Violation Examples**:
- Basket creation bugs
- Weight calculation errors
- Address validation failures

## Integration with Crisis Module

The invariants are registered with the Cosmos SDK crisis module, which can:

1. **Periodic Checks**: Run invariants at specified intervals
2. **On-Demand Checks**: Triggered by governance or operators
3. **Chain Halt**: Optionally halt the chain if invariants fail (configurable)

### Configuration

In `app.go`, the crisis module is configured to use LST invariants:

```go
// LST module registers its invariants automatically
app.LSTKeeper = lst.NewKeeper(...)
app.ModuleManager = module.NewManager(
    // ... other modules
    lst.NewAppModule(encodingConfig.Codec, app.LSTKeeper),
    // ...
)
```

### Running Invariants

**CLI Commands** (when crisis module is enabled):
```bash
# Check all invariants
celestia-appd tx crisis invariant-check lst basket-accounting --from validator

# Check specific invariant
celestia-appd tx crisis invariant-check lst module-accounts --from validator
```

**Programmatic Checks**:
```go
// In tests or debugging
results := keeper.CheckAllInvariants(ctx)
for _, result := range results {
    if result.Broken {
        fmt.Printf("Invariant %s failed: %s\n", result.Name, result.Msg)
    }
}
```

## Testing Invariants

The module includes comprehensive testing utilities for invariants:

### Test Scenarios

```go
// Get all test scenarios
scenarios := keeper.GetInvariantTestScenarios()

// Run specific scenario
result := keeper.RunInvariantTestScenario(ctx, scenario, basketID)
```

### Available Test Corruptions

1. **Basket Accounting**:
   - `inflate_total_staked`: Artificially increase total staked
   - `deflate_total_staked`: Artificially decrease total staked
   - `negative_shares`: Set negative share amounts
   - `unreasonable_exchange_rate`: Create invalid exchange rates

2. **Module Accounts**:
   - `excess_balance`: Add excessive module account balance
   - `main_module_excess`: Exceed main module account limits

3. **Pending Redemptions**:
   - `invalid_basket_ref`: Reference non-existent baskets
   - `negative_shares`: Invalid redemption amounts
   - `excessive_shares`: Redeem more than basket total

4. **Basket State**:
   - `duplicate_validators`: Add duplicate validators
   - `invalid_weights_sum`: Weights don't sum to 1.0
   - `wrong_denom`: Incorrect basket denomination

### Example Test Usage

```go
// Test basket accounting invariant
err := keeper.BreakBasketAccounting(ctx, basketID, "inflate_total_staked")
require.NoError(t, err)

// Check that invariant catches the violation
results := keeper.CheckAllInvariants(ctx)
found := false
for _, result := range results {
    if result.Name == "basket-accounting" && result.Broken {
        found = true
        break
    }
}
require.True(t, found, "Expected basket-accounting invariant to be broken")
```

## Operational Guidelines

### Production Deployment

1. **Enable Crisis Module**: Include crisis module in production builds
2. **Configure Checking**: Set appropriate invariant check intervals
3. **Monitor Alerts**: Set up monitoring for invariant failures
4. **Response Procedures**: Define procedures for invariant violations

### Recommended Configuration

```go
// In genesis or via governance
crisisParams := crisis.Params{
    ConstantFee: sdk.NewCoin("utia", math.NewInt(1000000)), // 1 TIA fee for invariant checks
}
```

### Monitoring

**Key Metrics**:
- Invariant check frequency
- Invariant failure rates
- Response times to violations

**Alerts**:
- Immediate alert on any invariant failure
- Weekly reports on invariant check results
- Performance impact monitoring

## Performance Considerations

### Computational Complexity

- **Basket Accounting**: O(baskets × validators_per_basket)
- **Module Accounts**: O(baskets)
- **Pending Redemptions**: O(pending_redemptions)
- **Basket State**: O(baskets × validators_per_basket)

### Resource Usage

- **Memory**: Proportional to number of baskets and validators
- **CPU**: Moderate, primarily query operations
- **Network**: Minimal, local state queries only

### Optimization

- Run invariants during low-activity periods
- Consider sampling for very large deployments
- Use async checking where possible

## Recovery Procedures

### Invariant Violation Response

1. **Immediate Actions**:
   - Stop new basket operations (if safe)
   - Alert operations team
   - Gather diagnostic information

2. **Investigation**:
   - Check recent transactions
   - Review validator slashing events
   - Examine module upgrade history

3. **Resolution**:
   - Fix underlying issues
   - Governance intervention if needed
   - Resume normal operations

### Common Fixes

- **Slashing Events**: Refresh basket values using `RefreshBasketTokenValue`
- **State Drift**: Use governance to correct recorded values
- **Module Account Issues**: Transfer stuck funds via governance

## Security Considerations

### Attack Vectors

- **State Manipulation**: Invariants help detect unauthorized state changes
- **Accounting Bugs**: Catch inflation/deflation of basket values
- **Denial of Service**: Monitor invariant check performance

### Best Practices

- Regular invariant checks in testnets
- Comprehensive test coverage for edge cases
- Clear escalation procedures for violations
- Automated monitoring and alerting

## Future Enhancements

### Planned Improvements

1. **Dynamic Thresholds**: Adjust tolerance based on network conditions
2. **Historical Tracking**: Record invariant check history
3. **Predictive Analysis**: Detect trends toward violations
4. **Recovery Automation**: Automatic fixes for common issues

### Integration Opportunities

1. **IBC Integration**: Cross-chain invariant checks
2. **Governance Integration**: Automatic parameter adjustments
3. **Slashing Integration**: Real-time slashing event handling
4. **Analytics Integration**: Export invariant data for analysis

## Conclusion

The LST module's invariant system provides comprehensive safety guarantees for liquid staking operations. By detecting state inconsistencies early, the system protects against bugs and ensures the integrity of basket token backing.

Regular invariant checking is essential for maintaining user confidence and preventing value leakage. The testing utilities enable thorough validation of the invariant system itself, ensuring it catches violations reliably.

Operators should integrate invariant checking into their monitoring and alerting systems to ensure prompt response to any detected issues.