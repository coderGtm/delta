package com.coderGtm.delta.outlet.mapper;

import org.springframework.stereotype.Component;

import com.coderGtm.delta.outlet.dto.OutletMembershipResponse;
import com.coderGtm.delta.outlet.dto.OutletResponse;
import com.coderGtm.delta.outlet.entity.Outlet;
import com.coderGtm.delta.outlet.entity.OutletMembership;
import com.coderGtm.delta.user.User;

/**
 * Maps outlet entities to stable API response payloads.
 */
@Component
public class OutletMapper {

	/**
	 * Converts an outlet entity into its external API representation.
	 */
	public OutletResponse toOutletResponse(Outlet outlet) {
		return new OutletResponse(
			outlet.getId(),
			outlet.getName(),
			outlet.getLatitude(),
			outlet.getLongitude(),
			outlet.getRadiusMeters(),
			outlet.getCreatedAt(),
			outlet.getUpdatedAt()
		);
	}

	/**
	 * Converts a membership entity into a response that includes both outlet and
	 * invited-by context to support owner and employee screens.
	 */
	public OutletMembershipResponse toMembershipResponse(OutletMembership membership) {
		User invitedBy = membership.getInvitedBy();

		return new OutletMembershipResponse(
			membership.getId(),
			toOutletResponse(membership.getOutlet()),
			membership.getUser().getId(),
			membership.getUser().getName(),
			membership.getUser().getEmail(),
			membership.getRole(),
			membership.getStatus(),
			invitedBy != null ? invitedBy.getId() : null,
			invitedBy != null ? invitedBy.getName() : null,
			membership.getCreatedAt(),
			membership.getUpdatedAt()
		);
	}
}
