package com.coderGtm.delta;

import org.springframework.boot.SpringBootConfiguration;
import org.springframework.boot.autoconfigure.EnableAutoConfiguration;
import org.springframework.context.annotation.ComponentScan;
import org.springframework.context.annotation.FilterType;
import org.springframework.data.jpa.repository.config.EnableJpaAuditing;
import org.springframework.scheduling.annotation.EnableScheduling;

import com.coderGtm.delta.config.FirebaseConfig;

@SpringBootConfiguration
@EnableAutoConfiguration
@EnableJpaAuditing(
	auditorAwareRef = "auditorAware",
	dateTimeProviderRef = "auditingDateTimeProvider"
)
@EnableScheduling
@ComponentScan(
	basePackages = "com.coderGtm.delta",
	excludeFilters = {
		@ComponentScan.Filter(type = FilterType.ASSIGNABLE_TYPE, classes = DeltaApplication.class),
		@ComponentScan.Filter(type = FilterType.ASSIGNABLE_TYPE, classes = FirebaseConfig.class)
	}
)
public class TestApplication {
}
