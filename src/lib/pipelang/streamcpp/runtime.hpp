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
// Incremental capabilities are borrowed only during a generated entry call.
// A host owns session lifetime, buffer storage, scheduling and serialization.
class StreamSession {
public:
    virtual ~StreamSession()=default;
};
struct InputBuffer { const void* data; std::size_t size; };
struct OutputBuffer { void* data; std::size_t capacity; };
enum class Progress : std::uint32_t { none=0, need_input=1, need_output=2, done=3 };
struct Step {
    Status status;
    Progress progress=Progress::none;
    std::uint64_t input_consumed=0, output_written=0;
    std::uint64_t input_bytes=0, output_bytes=0, chunks=0;
};
inline Step normalized(Step step) noexcept {
    if(normalized(step.status)!=step.status || (step.status==Status::ok &&
       (step.progress<Progress::need_input || step.progress>Progress::done)))
        return {Status::host_failure};
    if(step.status!=Status::ok) step.progress=Progress::none;
    return step;
}
class Host {
public:
    virtual ~Host()=default;
    // Native adapters are trusted code. Metadata alone grants no host authority.
    virtual Result invoke(const Binding&, ReadStream&, WriteStream&, std::uint64_t)=0;
    // Old hosts explicitly refuse incremental operations until they implement it.
    virtual Step invoke_step(const Binding&, StreamSession&, InputBuffer, OutputBuffer, bool) {
        return {Status::unsupported};
    }
};
inline bool valid_buffers(InputBuffer input, OutputBuffer output) noexcept {
    if((!input.data && input.size) || (!output.data && output.capacity)) return false;
    auto a=reinterpret_cast<std::uintptr_t>(input.data), b=reinterpret_cast<std::uintptr_t>(output.data);
    if(input.size>UINTPTR_MAX-a || output.capacity>UINTPTR_MAX-b) return false;
    return !input.size || !output.capacity || a+input.size<=b || b+output.capacity<=a;
}
inline Step call_step(Host& host, const Binding& binding, StreamSession& session,
                      InputBuffer input, OutputBuffer output, bool end_of_input) noexcept {
    if(!valid_buffers(input,output)) return {Status::invalid_argument};
    try {
        auto step=normalized(host.invoke_step(binding,session,input,output,end_of_input));
        if(step.input_consumed>input.size || step.output_written>output.capacity ||
           step.input_consumed>step.input_bytes || step.output_written>step.output_bytes)
            return {Status::host_failure};
        return step;
    } catch(...) { return {Status::host_failure}; }
}
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
