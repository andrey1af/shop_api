package repository

import "uuid"

const ShardCount = 4

func shardIndex(id uuid.UUID) int {
	firstHexDigit := int(id[0] >> 4)

	return firstHexDigit * ShardCount / 16
}
