# blobs_unpack.py
# 사용법: python blobs_unpack.py blob1.bin blob2.bin ... > decoded.bin
import sys

def unpack_blob(path):
    out = bytearray()
    b = open(path, "rb").read()
    if len(b) % 32 != 0:
        raise SystemExit(f"{path}: size is not multiple of 32B")
    for i in range(0, len(b), 32):
        out += b[i:i+31]   # 각 32바이트 필드에서 상위 31바이트만 취함
    return bytes(out)

if __name__ == "__main__":
    if len(sys.argv) < 2:
        raise SystemExit("usage: python blobs_unpack.py <blob1> [blob2 ...] > decoded.bin")
    buf = bytearray()
    for p in sys.argv[1:]:
        buf += unpack_blob(p)
    sys.stdout.buffer.write(buf)
