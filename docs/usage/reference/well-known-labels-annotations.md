# Well-Known Labels and Annotations

This document serves both as a reference to the values and as a coordination point for assigning values.

## Labels and annotations used on API objects

### seed.gardener.cloud/provider

**Type**: Label

**Example**: `seed.gardener.cloud/provider: "aws"`

**Used on**: `Seed` Objects

Identifies the seed provider's type. It can be used to configure a seed selector for the shoot.

### seed.gardener.cloud/region

**Type**: Label

**Example**: `seed.gardener.cloud/region: "us-east-1"`

**Used on**: `Seed` Objects

Identifies the seed provider's region. It can be used to configure a seed selector for the shoot.