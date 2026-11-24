#include "DockpipeSDKBridge.hpp"
#ifdef _WIN32
#include "WindowsFiles.hpp"
#endif
#include <algorithm>
#include <chrono>
#include <iostream>
#include <stdexcept>
#include <thread>
#include <vector>

void require(bool condition, const std::string& message) {
    if (!condition)
        throw std::runtime_error(message);
}

void checkCodec() {
    const std::string valid = R"({"kind":"workflow","root":"/tmp","scope":"test","workdir":"/tmp","name":"\ud83d\ude00"})";
    const auto decoded = dockpipe::sdk::decodeScopeObject(valid, valid.size());
    require(decoded.is_success(), "valid Unicode scope object was rejected");
    require(decoded.value().at("name") == "\xf0\x9f\x98\x80", "surrogate pair decoding changed bytes");
    require(!dockpipe::sdk::decodeScopeObject(valid, valid.size() - 1).is_success(), "byte budget was ignored");
    for (const std::string& invalid : std::vector<std::string>{
        "{}", "{", "[]", "null", "{\"kind\":42}",
        "{\"kind\":\"workflow\",\"kind\":\"package\"}",
        "{\"name\":\"\\ud800\"}", "{\"name\":\"\\udc00\"}",
        "{\"name\":\"\xc0\x80\"}", valid + "x"}) {
        require(!dockpipe::sdk::decodeScopeObject(invalid, 32768).is_success(), "malformed scope JSON was admitted");
    }
}

void diagnosticText(const char* name, const std::string& value, std::size_t maximumPrefixBytes) {
    constexpr char hexadecimal[] = "0123456789abcdef";
    const auto bytes = std::min(value.size(), maximumPrefixBytes);
    std::cerr << name << " bytes=" << value.size() << " prefix_bytes=" << bytes << " text=\"";
    for (std::size_t index = 0; index < bytes; ++index) {
        const auto byte = static_cast<unsigned char>(value[index]);
        if (byte >= 32 && byte <= 126 && byte != '\\' && byte != '"') {
            std::cerr.put(static_cast<char>(byte));
        } else {
            std::cerr << "\\x" << hexadecimal[byte >> 4] << hexadecimal[byte & 15];
        }
    }
    std::cerr << "\"\n";
}

void transportFailureDiagnostic(const dockpipe::sdk::Client& client) {
    // Snapshot only. Diagnostics do not observe, interrupt, dispose, retry or
    // decide a source assertion. Every printed text prefix has a fixed bound.
    const auto receipts = client.receipts();
    std::cerr << "SDK_TRANSPORT_FAILURE receipt_count=" << receipts.size() << '\n';
    if (receipts.empty())
        return;

    const auto& receipt = receipts.back();
    const auto& observation = receipt.observation;
    std::cerr << "latest_operation client_id=" << observation.id.clientId
              << " operation=" << observation.id.value
              << " root_pid=" << receipt.leaderPid
              << " scope_query=" << receipt.scopeQuery
              << " path_query=" << receipt.scopePathQuery
              << " exited=" << observation.exited
              << " tree_complete=" << observation.treeComplete
              << " forced=" << observation.forced
              << " exit_code=" << observation.exitCode
              << " terminated_children=" << observation.reapedChildren
              << " interrupt_requested=" << receipt.interruptRequested
              << " stdout_truncated=" << observation.stdoutTruncated
              << " stderr_truncated=" << observation.stderrTruncated << '\n';
    diagnosticText("infrastructure_failure", receipt.infrastructureFailure, 4096);
    diagnosticText("stdout", observation.stdoutText, 8192);
    diagnosticText("stderr", observation.stderrText, 8192);
    std::cerr << "argument_count=" << receipt.arguments.size() << '\n';
    const auto argumentCount = std::min<std::size_t>(receipt.arguments.size(), 8);
    for (std::size_t index = 0; index < argumentCount; ++index) {
        std::cerr << "argument_index=" << index << ' ';
        diagnosticText("argument", receipt.arguments[index], 512);
    }
}

int main(int argc, char** argv) {
    try {
#ifdef _WIN32
        require(argc == 6, "select CLI, proof root, failure and held fixtures, and console signal helper");
#else
        require(argc == 5, "select CLI, proof root, failure and held fixtures");
#endif
        checkCodec();
        std::string proofRoot = argv[2];
        std::string failureWorkflow = argv[3];
        std::string heldWorkflow = argv[4];
#ifdef _WIN32
        // Keep the actual admitted objects and give source their stable paths.
        // Scope results remain the CLI's public canonical paths.
        auto proofDirectory = dockpipe::sdk::openWindowsPath(proofRoot, true);
        auto failureFile = dockpipe::sdk::openWindowsPath(failureWorkflow, false);
        auto heldFile = dockpipe::sdk::openWindowsPath(heldWorkflow, false);
        proofRoot = dockpipe::sdk::windowsUtf8(proofDirectory.nativePath);
        failureWorkflow = dockpipe::sdk::windowsUtf8(failureFile.nativePath);
        heldWorkflow = dockpipe::sdk::windowsUtf8(heldFile.nativePath);
#endif

        dockpipe::sdk::ClientConfiguration configuration;
        configuration.executable = argv[1];
        configuration.allowedWorkdirRoot = proofRoot;
        configuration.stateDirectory = proofRoot + "/state";
        configuration.cacheDirectory = proofRoot + "/cache";
        configuration.maximumStoredRuns = 64;
        configuration.maximumActiveRuns = 1;
        configuration.maximumOutputBytesPerStream = 32768;
        configuration.maximumInventoryBytes = 1024 * 1024;
        configuration.maximumArgumentBytes = 65536;
#ifdef _WIN32
        configuration.consoleSignalExecutable = argv[5];
#endif
        auto client = std::make_shared<dockpipe::sdk::Client>(configuration);
        auto transport = std::make_shared<sdk_proof_host::Transport>(client);
        {
            sdk_proof::source::Dockpipe::SDK::Proof::Probe model(transport);
            pipelang::model::CancellationSource cancellation;
            pipelang::model::CancellationSource cancelled;
            cancelled.cancel();
            auto task = model.Run(proofRoot, failureWorkflow, heldWorkflow, cancellation.token(), cancelled.token());
            const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(30);
            while (!task.completed()) {
                model.poll();
                require(std::chrono::steady_clock::now() < deadline, "source proof watchdog expired");
                std::this_thread::sleep_for(std::chrono::milliseconds(1));
            }
            if (!task.result().is_success())
                transportFailureDiagnostic(*client);
            require(task.result().is_success(), task.result().error());
        }
        require(!client->hasUnreapedTree(), "native client retained a tree");
        require(!client->emergencyDispose(), "native proof required emergency disposal");
        sdk_proof_host::source::ScopeRequest oversizedScope{};
        oversizedScope.workdir = proofRoot;
        oversizedScope.scope.assign(65536, 'x');
        oversizedScope.outputLimitBytes = 32768;
        const auto oversized = transport->Scope(std::move(oversizedScope), 5000, {});
        require(oversized.is_failure(), "oversized scope request was admitted");
        const auto receipts = client->receipts();
        require(receipts.size() == 9, "source proof emitted an unexpected invocation count");
        const std::vector<std::string> expectedArguments{
            argv[1], "--workflow-file", failureWorkflow, "--workdir", proofRoot,
            "--var", "proof=spaces ; $(literal)", "--no-vault", "--", "argument with spaces"};
        require(receipts[5].arguments == expectedArguments, "workflow arguments changed token boundaries");
        require(receipts[6].interruptRequested, "graceful cancellation lacks an interrupt receipt");
        for (std::size_t index = 0; index < receipts.size(); ++index) {
            const auto& receipt = receipts[index];
            require(receipt.observation.treeComplete, "receipt lacks complete closure");
            require(receipt.observation.forced == (index == 7), "unexpected force disposal receipt");
            if (receipt.observation.stdoutTruncated || receipt.observation.stderrTruncated) {
                require(receipt.scopeQuery && !receipt.infrastructureFailure.empty(),
                    "truncation negative control lost its failure receipt");
            } else {
                require(receipt.infrastructureFailure.empty(), receipt.infrastructureFailure);
            }
        }
        std::cout << "source SDK scope and lifecycle proof passed; codec negative controls passed\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
