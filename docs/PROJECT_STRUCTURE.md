# zen-gc Project Structure

## Overview

`zen-gc` is a production-ready Kubernetes controller for automatic garbage collection of any Kubernetes resource.

## Project Goals

1. **Provide Value**: Solve the real-world problem of resource cleanup in Kubernetes
2. **Production Ready**: Build a reliable, observable, and maintainable solution
3. **Community Driven**: Open source project that welcomes contributions and feedback
4. **Future**: If widely adopted, may be proposed as a Kubernetes Enhancement Proposal (KEP)

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

## Development Status

### Current Status: Production Ready ✅

- ✅ Full-featured GC controller implementation
- ✅ Comprehensive CRD with flexible TTL and condition support
- ✅ Production features (rate limiting, metrics, HA)
- ✅ Extensive test coverage (>65%)
- ✅ Complete documentation
- ✅ Open source and actively maintained

### Future Considerations

If zen-gc gains significant community adoption and proves valuable to the Kubernetes ecosystem, it may be proposed as a Kubernetes Enhancement Proposal (KEP) to potentially become part of upstream Kubernetes. However, this is not the primary goal—the focus is on providing a useful, production-ready solution.

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

