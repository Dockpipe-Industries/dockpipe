"""Binary-only client for the separately supplied Nucleon experimental C ABI.

No codec implementation or model data belongs in this repository. The SDK is
loaded from a sealed, hash-verified copy and requires ABI version 1.
"""
import ctypes
import _ctypes
import os

from shared_runtime_probe import seal
from transcript_runtime import check


class SDK:
    LIMIT = 64 << 20

    def __init__(self, library, expected):
        self.lib = None
        self.fd = seal(library, expected)
        try:
            self.lib = ctypes.CDLL(f'/proc/self/fd/{self.fd}')
            self.lib.nucleon_abi_version.restype = ctypes.c_uint32
            check(self.lib.nucleon_abi_version() == 1, 'unsupported Nucleon SDK ABI')
            self.lib.nucleon_version.restype = ctypes.c_char_p
            self.lib.nucleon_last_error.restype = ctypes.c_char_p
            self.lib.nucleon_compress_bound.argtypes = [ctypes.c_size_t]
            self.lib.nucleon_compress_bound.restype = ctypes.c_size_t
            self.lib.nucleon_decoded_size.argtypes = [ctypes.c_void_p, ctypes.c_size_t, ctypes.POINTER(ctypes.c_size_t)]
            self.lib.nucleon_decoded_size.restype = ctypes.c_int
            for name in ['nucleon_encode', 'nucleon_decode']:
                function = getattr(self.lib, name)
                function.argtypes = [ctypes.c_void_p, ctypes.c_size_t, ctypes.c_void_p,
                                     ctypes.c_size_t, ctypes.POINTER(ctypes.c_size_t)]
                function.restype = ctypes.c_int
        except BaseException:
            self.close()
            raise

    def check(self, status):
        if status:
            raise ValueError('Nucleon SDK: ' + self.lib.nucleon_last_error().decode('utf-8', errors='replace'))

    def encode(self, source):
        check(len(source) <= self.LIMIT, 'Nucleon input limit')
        bound = self.lib.nucleon_compress_bound(len(source))
        check(24 <= bound <= self.LIMIT + 16408, 'Nucleon encode bound')
        output = ctypes.create_string_buffer(bound)
        written = ctypes.c_size_t()
        self.check(self.lib.nucleon_encode(source, len(source), output, bound, ctypes.byref(written)))
        check(written.value <= bound, 'Nucleon output extent')
        return output.raw[:written.value]

    def decode_into(self, packed, output, expected_size):
        check(0 < expected_size <= self.LIMIT and 24 <= len(packed) <= self.LIMIT + 16408, 'Nucleon decode bounds')
        size = ctypes.c_size_t()
        self.check(self.lib.nucleon_decoded_size(packed, len(packed), ctypes.byref(size)))
        check(size.value == expected_size, 'Nucleon decoded size mismatch')
        written = ctypes.c_size_t()
        self.check(self.lib.nucleon_decode(packed, len(packed), output, expected_size, ctypes.byref(written)))
        check(written.value == expected_size, 'Nucleon decoded extent')

    def close(self):
        if self.lib is not None:
            # Unload before recycling the fd: dlopen caches /proc/self/fd paths.
            # A later SDK with different bytes must never reuse an old handle.
            _ctypes.dlclose(self.lib._handle)
            self.lib = None
        if self.fd is not None:
            os.close(self.fd)
            self.fd = None
