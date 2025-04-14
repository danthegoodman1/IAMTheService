In this codebase, i am attempt to make an HTTP proxy that can optionally intercept AWS Service requests (e.g. to S3) and can act on them (if there is no interceptor for the specific request, it will re-sign and proxy the request). It can act on them by intercepting and returning instead of hitting the origin service (e.g. a nvme cache), it can mutate the request before sending to the origin (e.g. modifying the S3 path to handle multi-tenant access to a single bucket), and more.

I've started in Go and Rust, but ignore the rust for now. I'd like to just focus on the Go for now.

One reason I want to focus on the Go is thinking about the AWS service intercepting. I'd like to be able to "register" service providers (`AWSServiceProvider`) like an `S3ServiceProvider` for example, and then methods within S3 can optionally be overloaded. If a method is not overloaded, then by default it will do the `ProxiedRequest.DoProxiedRequest()` which re-signs the request with AWS SigV4 and forwards it to an origin. Info like key secrets, client to service provider, and service provider to target hostname are implemented with the `LookupProvider` (although I feel like it can be simplified, but that's a later stag).

This is only partially implemented as you can see in the codebase, but a few things I'd like to do are:

1. Finish the handling such that "unhandled" requests (non-overloaded methods for a provider) are forwarded to the origin service and proxied back properly (resigned).
2. Think about how to handle defining method overloads in a developer friendly way, and implement for an `S3ServiceProvider` as a first pass.

Point 2 has a couple things to consider:

The first is that what does the DX look like? I can imagine it's either like an HTTP framework where you match by some pattern of the request, or it's entirely method based where you'd pass it a function pointer to the `GetObject` property and it would invoke that instead of the `DoProxiedRequest` default (note that `GetObject` could still interally call `DoProxiedRequest` if it wants the response from the origin, and can either pass that response directly or mutate it before responding).

I feel like the later is more developer friendly, since not only are there specific methods to write handlers for, but we could also use the aws s3 go package to potentially rebuild the request into the pre-made structs that exist in the package like `github.com/aws/aws-sdk-go-v2/service/s3.PutObjectInput`. However this could result in a ton of boilerplate code of just checking methods, checking if the function to overload exists, and binding them.

The former would be a lot less (potential fragile) code that checks for binding to a method, but requires that the developer does a proper match. They should be able to look up the AWS service docs (e.g. go to https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html and see that it's `PUT /Key+` to match), and have to bind the request themselves (e.g. get the query params), but that can probably be more nicely abstracted, and results in a simpler package that is probably easier to unit test as well.

I need help with these two things (and for point 2, deciding which method to implement)
