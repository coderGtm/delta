package com.coderGtm.delta.attendance.repository;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

import org.springframework.data.jpa.repository.EntityGraph;
import org.springframework.data.jpa.repository.JpaRepository;

import com.coderGtm.delta.attendance.entity.AttendanceEntry;

/**
 * Repository for persisting and querying attendance entries.
 */
public interface AttendanceEntryRepository extends JpaRepository<AttendanceEntry, UUID> {

	@EntityGraph(attributePaths = {"user", "outlet"})
	List<AttendanceEntry> findAllByOutlet_IdOrderByEntryTimeDescCreatedAtDesc(UUID outletId);

	@EntityGraph(attributePaths = {"user", "outlet"})
	List<AttendanceEntry> findAllByOutlet_IdAndUser_IdOrderByEntryTimeDescCreatedAtDesc(UUID outletId, UUID userId);

	@EntityGraph(attributePaths = {"user", "outlet"})
	Optional<AttendanceEntry> findDetailedByIdAndOutlet_Id(UUID id, UUID outletId);
}
