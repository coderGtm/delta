package com.coderGtm.delta.outlet.entity;

import java.time.Instant;
import java.util.UUID;

import com.coderGtm.delta.common.entity.BaseEntity;
import com.coderGtm.delta.user.User;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.FetchType;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.JoinColumn;
import jakarta.persistence.ManyToOne;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * Explicit join entity between {@link User} and {@link Outlet}.
 *
 * <p>This entity stores both the user's role in the outlet and the lifecycle
 * of the membership invitation. Using an explicit entity instead of a simple
 * many-to-many relationship keeps the domain flexible and allows invitation
 * state transitions to be modelled cleanly.</p>
 */
@Entity
@Table(
	name = "outlet_memberships",
	uniqueConstraints = {
		@UniqueConstraint(name = "uk_outlet_membership_outlet_user", columnNames = {"outlet_id", "user_id"})
	}
)
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class OutletMembership extends BaseEntity {

	@Id
	@GeneratedValue(strategy = GenerationType.UUID)
	private UUID id;

	@ManyToOne(fetch = FetchType.LAZY, optional = false)
	@JoinColumn(name = "outlet_id", nullable = false)
	private Outlet outlet;

	@ManyToOne(fetch = FetchType.LAZY, optional = false)
	@JoinColumn(name = "user_id", nullable = false)
	private User user;

	/**
	 * Owner-controlled per-outlet name used to identify this member to humans in
	 * frontend lists. Defaults to the user's account name at membership
	 * creation.
	 */
	@Column(nullable = false, length = 255)
	private String displayName;

	@Enumerated(EnumType.STRING)
	@Column(nullable = false, length = 20)
	private OutletRole role;

	@Enumerated(EnumType.STRING)
	@Column(nullable = false, length = 20)
	private OutletMembershipStatus status;

	@ManyToOne(fetch = FetchType.LAZY)
	@JoinColumn(name = "invited_by_user_id")
	private User invitedBy;

	/**
	 * Soft-delete timestamp used when an owner removes a membership while keeping
	 * the historical relationship intact for auditability and future attendance
	 * references.
	 */
	private Instant removedAt;

	@ManyToOne(fetch = FetchType.LAZY)
	@JoinColumn(name = "removed_by_user_id")
	private User removedBy;
}
