"""Binary ABI adapter checks without requiring the private codec implementation."""
import ctypes
import tracemalloc
from types import SimpleNamespace
import unittest

from nucleon_sdk import SDK


class SDKBufferTests(unittest.TestCase):
    def adapter(self, payload, bound, reported_size=None):
        def encode(source, size, output, capacity, written):
            ctypes.memmove(output, payload, len(payload))
            ctypes.cast(written, ctypes.POINTER(ctypes.c_size_t))[0] = (
                len(payload) if reported_size is None else reported_size)
            return 0
        sdk = object.__new__(SDK)
        sdk.lib = SimpleNamespace(nucleon_compress_bound=lambda size: bound,
                                  nucleon_encode=encode)
        return sdk

    def test_exact_binary_output_does_not_copy_unused_capacity(self):
        payload = b'header\x00binary\xfftail'
        bound = 1 << 20
        sdk = self.adapter(payload, bound)
        tracemalloc.start()
        try:
            self.assertEqual(sdk.encode(b'input'), payload)
            _, peak = tracemalloc.get_traced_memory()
        finally:
            tracemalloc.stop()
        # One bounded native destination plus small adapter overhead; a second
        # full-capacity Python bytes object would exceed this allowance.
        self.assertLess(peak, bound + (256 << 10))

    def test_invalid_written_extent_is_rejected_before_read(self):
        sdk = self.adapter(b'x', 32, reported_size=33)
        with self.assertRaisesRegex(ValueError, 'output extent'):
            sdk.encode(b'input')


if __name__ == '__main__': unittest.main()
