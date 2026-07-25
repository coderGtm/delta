package com.coderGtm.delta.common.dto;

import java.util.List;

/**
 * Generic paginated API response wrapper used by collection endpoints.
 */
public record PageResponse<T>(
	List<T> content,
	int page,
	int size,
	long totalElements,
	int totalPages,
	boolean first,
	boolean last,
	boolean empty
) {
}
