package com.coderGtm.delta.common.audit.service;

import java.util.Map;
import java.util.UUID;

import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.context.request.RequestAttributes;
import org.springframework.web.context.request.RequestContextHolder;
import org.springframework.web.context.request.ServletRequestAttributes;

import com.coderGtm.delta.common.audit.entity.AuditEvent;
import com.coderGtm.delta.common.audit.repository.AuditEventRepository;
import com.coderGtm.delta.common.web.ClientIpUtils;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;

import jakarta.servlet.http.HttpServletRequest;
import lombok.RequiredArgsConstructor;

/**
 * Persists business audit events in an append-only table.
 */
@Service
@RequiredArgsConstructor
public class AuditService {

	private final AuditEventRepository auditEventRepository;
	private final ObjectMapper objectMapper = new ObjectMapper();

	/**
	 * Records a new audit event with request metadata when available.
	 */
	@Transactional(propagation = Propagation.REQUIRES_NEW)
	public void record(UUID actorUserId, String action, String entityType, UUID entityId, Map<String, Object> metadata) {
		AuditEvent event = new AuditEvent();
		event.setActorUserId(actorUserId);
		event.setAction(action);
		event.setEntityType(entityType);
		event.setEntityId(entityId);
		event.setMetadataJson(toJson(metadata));

		HttpServletRequest request = currentRequest();
		if (request != null) {
			event.setIpAddress(ClientIpUtils.resolve(request));
			event.setUserAgent(truncate(request.getHeader("User-Agent"), 500));
		}

		auditEventRepository.save(event);
	}

	private HttpServletRequest currentRequest() {
		RequestAttributes requestAttributes = RequestContextHolder.getRequestAttributes();
		if (requestAttributes instanceof ServletRequestAttributes servletRequestAttributes) {
			return servletRequestAttributes.getRequest();
		}
		return null;
	}

	private String toJson(Map<String, Object> metadata) {
		if (metadata == null || metadata.isEmpty()) {
			return null;
		}

		try {
			return objectMapper.writeValueAsString(metadata);
		} catch (JsonProcessingException e) {
			throw new IllegalStateException("Failed to serialize audit event metadata", e);
		}
	}

	private String truncate(String value, int maxLength) {
		if (value == null || value.length() <= maxLength) {
			return value;
		}
		return value.substring(0, maxLength);
	}
}
