package com.coderGtm.delta.common.util;

import java.util.function.Function;

import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Pageable;
import org.springframework.data.domain.Sort;

import com.coderGtm.delta.common.dto.PageResponse;

/**
 * Shared pagination helpers for building stable paged API responses.
 */
public final class PaginationUtils {

	private PaginationUtils() {
	}

	/**
	 * Applies a default sort when the incoming pageable does not explicitly
	 * request one.
	 */
	public static Pageable withDefaultSort(Pageable pageable, Sort defaultSort) {
		if (pageable.getSort().isSorted()) {
			return pageable;
		}

		return PageRequest.of(pageable.getPageNumber(), pageable.getPageSize(), defaultSort);
	}

	/**
	 * Maps a Spring Data page into the application's generic page response
	 * payload.
	 */
	public static <T, R> PageResponse<R> toPageResponse(Page<T> page, Function<T, R> mapper) {
		return new PageResponse<>(
			page.getContent().stream().map(mapper).toList(),
			page.getNumber(),
			page.getSize(),
			page.getTotalElements(),
			page.getTotalPages(),
			page.isFirst(),
			page.isLast(),
			page.isEmpty()
		);
	}
}
