python blobs_unpack.py blob_0.bin blob_1.bin blob_2.bin > decoded.bin

go build -o opstack-decoder

./opstack-decoder ../blobs_unpack/decoded.bin
