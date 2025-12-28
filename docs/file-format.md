# File Metadata Format

Each file is split into fixed-size chunks (1MB default).
Each chunk is hashed using SHA-256.

## File ID
The file ID is computed as a SHA-256 hash of all chunk hashes
concatenated in order. This ensures content-addressed identity.

## Resume Support
Downloaded chunks are stored individually and tracked via state.json.
