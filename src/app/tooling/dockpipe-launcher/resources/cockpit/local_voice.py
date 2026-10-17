"""Local, bounded push-to-talk commands using the optional Linux speech packages.

No network, audio files, shell, or arbitrary-command execution. The parent owns
this isolated helper and kills it after 15 seconds if audio/decoder I/O stalls.
PocketSphinx 0.8's packaged ABI is selected explicitly by its .so.3 soname.
"""
import ctypes as c
import json
from pathlib import Path
import sys
import threading
import time

COMMANDS = (
    "open files", "open apps", "open workflows", "open containers",
    "open settings", "open machines", "open executions", "return to orbit",
    "close panel", "full screen", "exit full screen", "reset hud",
    "open workspace", "next workspace",
)
MODEL = Path("/usr/share/pocketsphinx/model/en-us")


def bind(library, name, result, arguments):
    function = getattr(library, name)
    function.restype = result
    function.argtypes = arguments
    return function


class Decoder:
    def __init__(self):
        self.ps = c.CDLL("libpocketsphinx.so.3")
        self.base = c.CDLL("libsphinxbase.so.3")
        for path in (MODEL / "en-us", MODEL / "cmudict-en-us.dict"):
            if not path.exists():
                raise RuntimeError("Install the local pocketsphinx-en-us speech model.")
        arguments = bind(self.ps, "ps_args", c.c_void_p, [])()
        config_init = bind(self.base, "cmd_ln_init", c.c_void_p, [c.c_void_p, c.c_void_p, c.c_int])
        config = config_init(None, arguments, 1,
            b"-hmm", str(MODEL / "en-us").encode(),
            b"-dict", str(MODEL / "cmudict-en-us.dict").encode(),
            b"-samprate", b"16000", b"-logfn", b"/dev/null", None)
        if not config:
            raise RuntimeError("Could not configure local recognition.")
        self.decoder = bind(self.ps, "ps_init", c.c_void_p, [c.c_void_p])(config)
        bind(self.base, "cmd_ln_free_r", c.c_int, [c.c_void_p])(config)
        if not self.decoder:
            raise RuntimeError("Could not load the local speech model.")
        grammar = "#JSGF V1.0; grammar cockpit; public <command> = " + " | ".join(COMMANDS) + ";"
        setup = bind(self.ps, "ps_set_jsgf_string", c.c_int, [c.c_void_p, c.c_char_p, c.c_char_p])
        select = bind(self.ps, "ps_set_search", c.c_int, [c.c_void_p, c.c_char_p])
        if setup(self.decoder, b"cockpit", grammar.encode()) < 0 or select(self.decoder, b"cockpit") < 0:
            self.close()
            raise RuntimeError("Could not configure cockpit command vocabulary.")
        self.start = bind(self.ps, "ps_start_utt", c.c_int, [c.c_void_p])
        self.process = bind(self.ps, "ps_process_raw", c.c_int,
            [c.c_void_p, c.POINTER(c.c_int16), c.c_size_t, c.c_int, c.c_int])
        self.end = bind(self.ps, "ps_end_utt", c.c_int, [c.c_void_p])
        self.hypothesis = bind(self.ps, "ps_get_hyp", c.c_char_p,
            [c.c_void_p, c.c_void_p, c.c_void_p])

    def close(self):
        if self.decoder:
            bind(self.ps, "ps_free", c.c_int, [c.c_void_p])(self.decoder)
            self.decoder = None


def listen(decoder):
    stopped = threading.Event()
    threading.Thread(target=lambda: (sys.stdin.readline(), stopped.set()), daemon=True).start()
    if stopped.is_set():
        return ""
    pulse = c.CDLL("libpulse-simple.so.0")

    class SampleSpec(c.Structure):
        _fields_ = [("format", c.c_int), ("rate", c.c_uint32), ("channels", c.c_uint8)]

    new = bind(pulse, "pa_simple_new", c.c_void_p,
        [c.c_char_p, c.c_char_p, c.c_int, c.c_char_p, c.c_char_p,
         c.POINTER(SampleSpec), c.c_void_p, c.c_void_p, c.POINTER(c.c_int)])
    read = bind(pulse, "pa_simple_read", c.c_int,
        [c.c_void_p, c.c_void_p, c.c_size_t, c.POINTER(c.c_int)])
    free = bind(pulse, "pa_simple_free", None, [c.c_void_p])
    error = c.c_int()
    stream = new(None, b"Dockpipe cockpit", 2, None, b"Push to talk",
        c.byref(SampleSpec(3, 16000, 1)), None, None, c.byref(error))
    if not stream:
        raise RuntimeError("Microphone unavailable. Check the desktop's audio input permissions.")
    try:
        if decoder.start(decoder.decoder) < 0:
            raise RuntimeError("Could not start recognition.")
        print(json.dumps({"status": "listening"}), flush=True)
        samples = (c.c_int16 * 1600)()
        deadline = time.monotonic() + 8
        while not stopped.is_set() and time.monotonic() < deadline:
            if read(stream, samples, c.sizeof(samples), c.byref(error)) < 0:
                raise RuntimeError("Microphone disconnected.")
            if decoder.process(decoder.decoder, samples, len(samples), 0, 0) < 0:
                raise RuntimeError("Recognition failed.")
        decoder.end(decoder.decoder)
        result = decoder.hypothesis(decoder.decoder, None, None)
        command = result.decode() if result else ""
        return command if command in COMMANDS else ""
    finally:
        free(stream)


def main():
    decoder = Decoder()
    try:
        if "--check" in sys.argv:
            c.CDLL("libpulse-simple.so.0")
            print(json.dumps({"status": "ready"}))
        elif "--self-test" in sys.argv:
            decoder.start(decoder.decoder)
            silence = (c.c_int16 * 16000)()
            decoder.process(decoder.decoder, silence, len(silence), 0, 1)
            decoder.end(decoder.decoder)
            assert not decoder.hypothesis(decoder.decoder, None, None)
            print(json.dumps({"status": "passed", "commands": len(COMMANDS)}))
        else:
            print(json.dumps({"command": listen(decoder)}), flush=True)
    finally:
        decoder.close()


if __name__ == "__main__":
    try:
        main()
    except (OSError, RuntimeError, AttributeError) as error:
        print(json.dumps({"error": str(error)}), flush=True)
        sys.exit(1)
