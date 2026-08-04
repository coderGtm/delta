package com.coderGtm.delta.common.web;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.content;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.redirectedUrl;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.web.context.WebApplicationContext;

import com.coderGtm.delta.TestApplication;
import com.coderGtm.delta.TestFirebaseConfiguration;

import jakarta.servlet.Filter;

@SpringBootTest(classes = { TestApplication.class, TestFirebaseConfiguration.class })
class OpenApiDocsIntegrationTest {

	@Autowired
	private WebApplicationContext context;

	@Autowired
	private Filter springSecurityFilterChain;

	private MockMvc mockMvc;

	@BeforeEach
	void setUp() {
		mockMvc = MockMvcBuilders.webAppContextSetup(context)
			.addFilters(springSecurityFilterChain)
			.build();
	}

	@Test
	void swaggerUiEntryPointRedirectsToGeneratedUi() throws Exception {
		mockMvc.perform(get("/docs"))
			.andExpect(status().is3xxRedirection())
			.andExpect(redirectedUrl("/swagger-ui/index.html"));
	}

	@Test
	void legacyDocsIndexUrlRedirectsToSwaggerUi() throws Exception {
		mockMvc.perform(get("/docs/index.html"))
			.andExpect(status().is3xxRedirection())
			.andExpect(redirectedUrl("/docs"));
	}

	@Test
	void swaggerUiHtmlIsServed() throws Exception {
		mockMvc.perform(get("/swagger-ui/index.html"))
			.andExpect(status().isOk())
			.andExpect(content().contentTypeCompatibleWith("text/html"));
	}

	@Test
	void openApiYamlIsGenerated() throws Exception {
		mockMvc.perform(get("/docs/openapi.yaml"))
			.andExpect(status().isOk())
			.andExpect(content().contentTypeCompatibleWith("application/vnd.oai.openapi"))
			.andExpect(content().string(org.hamcrest.Matchers.containsString("Delta API")))
			.andExpect(content().string(org.hamcrest.Matchers.containsString("bearerAuth")));
	}

	@Test
	void openApiJsonIsGenerated() throws Exception {
		mockMvc.perform(get("/docs/openapi"))
			.andExpect(status().isOk())
			.andExpect(content().contentTypeCompatibleWith("application/json"));
	}
}
