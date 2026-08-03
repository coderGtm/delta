package com.coderGtm.delta.common.metrics;

import org.springframework.stereotype.Component;

import io.micrometer.core.instrument.MeterRegistry;
import lombok.RequiredArgsConstructor;

/**
 * Small wrapper around Micrometer used to emit business-level metrics from
 * services without repeating meter wiring logic everywhere.
 */
@Component
@RequiredArgsConstructor
public class ApplicationMetrics {

	private final MeterRegistry meterRegistry;

	/**
	 * Increments a named counter with optional tag key/value pairs.
	 */
	public void increment(String counterName, String... tags) {
		meterRegistry.counter(counterName, tags).increment();
	}
}
