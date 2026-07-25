package com.coderGtm.delta.attendance.entity;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.UUID;

import org.springframework.data.annotation.CreatedBy;
import org.springframework.data.annotation.LastModifiedBy;

import com.coderGtm.delta.common.entity.BaseEntity;
import com.coderGtm.delta.outlet.entity.Outlet;
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
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * Historical attendance entry recorded against a user and outlet.
 *
 * <p>The entry keeps direct references to the user and outlet so attendance
 * history remains valid even if the user's outlet membership is soft-removed
 * later.</p>
 */
@Entity
@Table(name = "attendance_entries")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class AttendanceEntry extends BaseEntity {

	@Id
	@GeneratedValue(strategy = GenerationType.UUID)
	private UUID id;

	@ManyToOne(fetch = FetchType.LAZY, optional = false)
	@JoinColumn(name = "user_id", nullable = false)
	private User user;

	@ManyToOne(fetch = FetchType.LAZY, optional = false)
	@JoinColumn(name = "outlet_id", nullable = false)
	private Outlet outlet;

	@Enumerated(EnumType.STRING)
	@Column(nullable = false, length = 20)
	private AttendanceEntryType type;

	@Column(nullable = false)
	private Instant entryTime;

	@Column(nullable = false, precision = 10, scale = 7)
	private BigDecimal latitude;

	@Column(nullable = false, precision = 10, scale = 7)
	private BigDecimal longitude;

	@CreatedBy
	@Column(name = "created_by")
	private UUID createdBy;

	@LastModifiedBy
	@Column(name = "updated_by")
	private UUID updatedBy;
}
