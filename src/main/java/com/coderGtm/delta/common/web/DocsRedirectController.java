package com.coderGtm.delta.common.web;

import org.springframework.stereotype.Controller;
import org.springframework.web.bind.annotation.GetMapping;

/**
 * Redirects the legacy documentation URL to the springdoc-generated Swagger UI
 * so existing bookmarks keep working.
 */
@Controller
public class DocsRedirectController {

	/**
	 * Redirects {@code /docs/index.html} to the Swagger UI entry point.
	 */
	@GetMapping("/docs/index.html")
	public String docsIndex() {
		return "redirect:/docs";
	}
}
