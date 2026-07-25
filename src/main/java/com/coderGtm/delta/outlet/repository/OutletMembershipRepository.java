package com.coderGtm.delta.outlet.repository;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

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
	List<OutletMembership> findAllByUser_IdAndStatusAndRemovedAtIsNullOrderByUpdatedAtDesc(
		UUID userId,
		OutletMembershipStatus status
	);

	@EntityGraph(attributePaths = {"outlet", "user", "invitedBy", "removedBy"})
	List<OutletMembership> findAllByOutlet_IdAndRemovedAtIsNullOrderByCreatedAtAsc(UUID outletId);
}
