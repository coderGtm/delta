package com.coderGtm.delta.common.audit.repository;

import java.util.UUID;

import org.springframework.data.jpa.repository.JpaRepository;

import com.coderGtm.delta.common.audit.entity.AuditEvent;

/**
 * Repository for persisting append-only audit events.
 */
public interface AuditEventRepository extends JpaRepository<AuditEvent, UUID> {
}
