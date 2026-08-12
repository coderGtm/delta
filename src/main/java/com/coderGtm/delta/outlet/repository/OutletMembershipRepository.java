package com.coderGtm.delta.outlet.repository;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.EntityGraph;
import org.springframework.data.jpa.repository.JpaRepository;

import com.coderGtm.delta.outlet.entity.OutletMembership;
import com.coderGtm.delta.outlet.entity.OutletMembershipStatus;

/**
 * Repository for outlet membership and invitation queries.
 */
public interface OutletMembershipRepository extends JpaRepository<OutletMembership, UUID> {

	Optional<OutletMembership> findByOutlet_IdAndUser_Id(UUID outletId, UUID userId);

	Optional<OutletMembership> findByOutlet_IdAndUser_IdAndRemovedAtIsNull(UUID outletId, UUID userId);

	@EntityGraph(attributePaths = {"outlet", "user", "invitedBy", "removedBy"})
	Optional<OutletMembership> findDetailedByIdAndRemovedAtIsNull(UUID id);

	@EntityGraph(attributePaths = {"outlet", "user", "invitedBy", "removedBy"})
	Page<OutletMembership> findAllByUser_IdAndStatusAndRemovedAtIsNullAndOutlet_RemovedAtIsNull(
		UUID userId,
		OutletMembershipStatus status,
		Pageable pageable
	);

	@EntityGraph(attributePaths = {"outlet", "user", "invitedBy", "removedBy"})
	Page<OutletMembership> findAllByOutlet_IdAndRemovedAtIsNull(UUID outletId, Pageable pageable);

	@EntityGraph(attributePaths = {"user"})
	List<OutletMembership> findAllByOutlet_Id(UUID outletId);
}
