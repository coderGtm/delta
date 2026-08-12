package com.coderGtm.delta.outlet.entity;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.UUID;

import com.coderGtm.delta.common.entity.BaseEntity;
import com.coderGtm.delta.user.User;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.FetchType;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.JoinColumn;
import jakarta.persistence.ManyToOne;
import jakarta.persistence.Table;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * Physical work location where employees can clock attendance entries.
 *
 * <p>The outlet stores its geofence center using latitude and longitude and a
 * configurable radius in meters. The radius is included now so the attendance
 * module can later validate whether an entry was created within the outlet's
 * allowed boundary.</p>
 */
@Entity
@Table(name = "outlets")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class Outlet extends BaseEntity {

	@Id
	@GeneratedValue(strategy = GenerationType.UUID)
	private UUID id;

	@Column(nullable = false, length = 150)
	private String name;

	@Column(nullable = false, precision = 10, scale = 7)
	private BigDecimal latitude;

	@Column(nullable = false, precision = 10, scale = 7)
	private BigDecimal longitude;

	@Column(nullable = false)
	private Integer radiusMeters;

	/**
	 * Controls whether attendance write requests must originate within the
	 * outlet's configured geofence.
	 */
	@Column(nullable = false)
	private boolean geofenceEnabled;

	/**
	 * Soft-delete timestamp used when an owner deletes the outlet while keeping
	 * historical attendance and membership records intact for reporting and
	 * auditability.
	 */
	private Instant removedAt;

	@ManyToOne(fetch = FetchType.LAZY)
	@JoinColumn(name = "removed_by_user_id")
	private User removedBy;
}
