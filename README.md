<p align="center">
  <img src="./docs/kitted-gopher.png" alt="A gangster Gopher that's kitted"/>
</p>

# Central Profile Build Service
A POC to highlight what a central profile build service would look like.

Profile-guided optimization (PGO);
> also known as feedback-directed optimization (FDO), is a compiler optimization technique that feeds information (a profile) from representative runs of the application back into to the compiler for the next build of the application, which uses that information to make more informed optimization decisions. For example, the compiler may decide to more aggressively inline functions which the profile indicates are called frequently.

[more here](https://go.dev/doc/pgo)

## Problem
In many orginizations, services can typically be many, can be ephemeral in nature; perhaps due to frequent deployments, or genuine underlying instance instability (such as instance updates, using [spot instances](https://aws.amazon.com/ec2/spot/) etc...). Containers typically host these services, completely disjoint from the build processes of the underlying service code. This separation provides a challenge for gathering runtime profiles and feeding them into the compiler.

## Proof of Concept;
A central server that handles the curation of runtime profiles, allows a space for complication in the form of using heuristics of a profile to determine it's suitability for use on the build side. As this service is up, the idea is to have participating services call on a client library in `main`;

```go
emit, _ := pcap.Capture("<service identifier>") // optionally handle setup err
defer func() {
  if err := emit(nil); err != nil {
    logger.WarnContext(ctx, "Failed to emit profile: "+err.Error())
  }
}() // optionally handle emit err
```
