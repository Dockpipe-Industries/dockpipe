#ifndef PIPELANG_NATIVE_STREAM_V1_HPP
#define PIPELANG_NATIVE_STREAM_V1_HPP
#include <cstddef>
#include <cstdint>
#include <string_view>

namespace pl_stream_v1 {
using Read = int (*)(void*, void*, std::size_t, std::size_t*);
using Write = int (*)(void*, const void*, std::size_t, std::size_t*);
// Host-owned capabilities, borrowed for one synchronous invocation. Source
// programs cannot construct, store, close, cast or asynchronously retain them.
struct ReadStream { Read read; void* context; };
struct WriteStream { Write write; void* context; };
enum class Status : std::uint32_t {
    ok=0, invalid_argument=1, invalid_stream=2, unsupported=3,
    limit_exceeded=4, io_error=5, host_failure=6, denied=7
};
struct Result {
    Status status;
    std::uint64_t input_bytes=0, output_bytes=0, chunks=0;
};
inline Status normalized(Status status) noexcept {
    return static_cast<std::uint32_t>(status)<=static_cast<std::uint32_t>(Status::denied)
        ? status : Status::host_failure;
}
inline Result normalized(Result result) noexcept {
    if(normalized(result.status)!=result.status)return {Status::host_failure};
    return result;
}
struct Binding {
    std::string_view package, operation, manifest_sha256;
};
class Host {
public:
    virtual ~Host()=default;
    // Native adapters are trusted code. Metadata alone grants no host authority.
    virtual Result invoke(const Binding&, ReadStream&, WriteStream&, std::uint64_t)=0;
};
inline Result call(Host& host, const Binding& binding, ReadStream& input,
                   WriteStream& output, std::int64_t max_bytes) noexcept {
    if(max_bytes<0 || !input.read || !output.write) return {Status::invalid_argument};
    try {
        auto result=host.invoke(binding,input,output,static_cast<std::uint64_t>(max_bytes));
        if(static_cast<std::uint32_t>(result.status)>static_cast<std::uint32_t>(Status::denied))
            return {Status::host_failure};
        return result;
    } catch(...) {return {Status::host_failure};}
}
}
#endif
