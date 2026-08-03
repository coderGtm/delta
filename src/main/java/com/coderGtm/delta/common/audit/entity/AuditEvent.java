package com.coderGtm.delta.common.audit.entity;

import java.util.UUID;

import com.coderGtm.delta.common.entity.BaseEntity;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * Append-only audit event that records business actions performed through the
 * API with enough metadata for later investigation.
 */
@Entity
@Table(name = "audit_events")
@Getter
@Setter
@NoArgsConstructor
public class AuditEvent extends BaseEntity {

	@Id
	@GeneratedValue(strategy = GenerationType.UUID)
	private UUID id;

	private UUID actorUserId;

	@Column(nullable = false, length = 100)
	private String action;

	@Column(nullable = false, length = 100)
	private String entityType;

	private UUID entityId;

	@Column(columnDefinition = "text")
	private String metadataJson;

	@Column(length = 100)
	private String ipAddress;

	@Column(length = 500)
	private String userAgent;
}
