package com.coderGtm.delta.attendance.repository;

import java.time.Instant;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.EntityGraph;
import org.springframework.data.jpa.repository.JpaRepository;

import com.coderGtm.delta.attendance.entity.AttendanceEntry;

/**
 * Repository for persisting and querying attendance entries.
 */
public interface AttendanceEntryRepository extends JpaRepository<AttendanceEntry, UUID> {

	@EntityGraph(attributePaths = {"user", "outlet"})
	Page<AttendanceEntry> findAllByOutlet_Id(UUID outletId, Pageable pageable);

	@EntityGraph(attributePaths = {"user", "outlet"})
	Page<AttendanceEntry> findAllByOutlet_IdAndUser_Id(UUID outletId, UUID userId, Pageable pageable);

	@EntityGraph(attributePaths = {"user", "outlet"})
	Optional<AttendanceEntry> findDetailedByIdAndOutlet_Id(UUID id, UUID outletId);

	@EntityGraph(attributePaths = {"user", "outlet"})
	List<AttendanceEntry> findAllByOutlet_IdAndUser_IdAndEntryTimeGreaterThanEqualAndEntryTimeLessThanOrderByEntryTimeAsc(
		UUID outletId,
		UUID userId,
		Instant startInclusive,
		Instant endExclusive
	);
}
