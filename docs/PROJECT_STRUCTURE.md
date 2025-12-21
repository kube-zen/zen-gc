# zen-gc Project Structure

## Overview

`zen-gc` is the implementation and PoC for the Generic Garbage Collection KEP proposal.

## Project Goals

1. **Implement PoC** of the Generic GC Controller based on the KEP
2. **Validate Design** through real-world testing
3. **Open Source** the implementation for community feedback
4. **Submit KEP** to Kubernetes Enhancement Proposals after validation

## Project Structure

```
zen-gc/
├── docs/
│   ├── KEP_GENERIC_GARBAGE_COLLECTION.md  # KEP document
│   ├── IMPLEMENTATION_ROADMAP.md          # Implementation plan
│   └── PROJECT_STRUCTURE.md               # This file
├── cmd/
│   └── gc-controller/                     # Main controller binary
├── pkg/
│   ├── controller/                        # GC controller implementation
│   ├── api/                               # GarbageCollectionPolicy CRD
│   └── validation/                        # GVR and policy validation
├── deploy/
│   ├── crds/                              # CRD definitions
│   └── manifests/                         # Deployment manifests
├── examples/                               # Example GC policies
├── test/                                   # Integration tests
└── README.md                               # Project overview
```

## Development Phases

### Phase 1: KEP Preparation (Current)
- ✅ KEP document drafted
- ⏳ Community feedback gathering
- ⏳ API design refinement

### Phase 2: PoC Implementation (After KEP Review)
- Implement basic controller
- CRD definition
- Basic TTL support
- Testing with kind/minikube

### Phase 3: Open Source Release
- Release as OSS project
- Community testing
- Gather feedback

### Phase 4: KEP Submission (After Validation)
- Submit to Kubernetes Enhancement Proposals
- SIG review process
- Iterate based on feedback

## Key Principles

1. **Kubernetes-Native**: Uses standard Kubernetes patterns
2. **Generic**: Works with any Kubernetes resource
3. **Community-Driven**: Open source, community feedback welcome
4. **Production-Ready**: Built-in rate limiting, metrics, and observability

## Next Steps

1. Review and refine KEP document
2. Gather initial community feedback
3. Implement PoC when ready
4. Open source release
5. Submit KEP after validation

