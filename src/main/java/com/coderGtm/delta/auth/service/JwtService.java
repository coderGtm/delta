package com.coderGtm.delta.auth.service;

import java.nio.charset.StandardCharsets;
import java.security.Key;
import java.util.Date;
import java.util.UUID;

import javax.crypto.SecretKey;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import com.coderGtm.delta.user.User;

import io.jsonwebtoken.Claims;
import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.security.Keys;

/**
 * Creates and validates the application's access tokens.
 */
@Service
public class JwtService {

	@Value("${jwt.secret}")
	private String secret;

	@Value("${jwt.access-token-expiration}")
	private long accessTokenExpiration;

	/**
	 * Generates a short-lived access token whose subject is the local user ID.
	 */
	public String generateAccessToken(User user) {
		return Jwts.builder()
			.subject(user.getId().toString())
			.issuedAt(new Date())
			.expiration(new Date(System.currentTimeMillis() + accessTokenExpiration))
			.signWith(getSigningKey())
			.compact();
	}

	/**
	 * Extracts the local user ID embedded in the token subject.
	 */
	public UUID extractUserId(String token) {
		Claims claims = parseClaims(token);
		return UUID.fromString(claims.getSubject());
	}

	/**
	 * Returns whether the token can be parsed and verified successfully.
	 */
	public boolean isTokenValid(String token) {
		try {
			parseClaims(token);
			return true;
		} catch (Exception e) {
			return false;
		}
	}

	private Key getSigningKey() {
		return Keys.hmacShaKeyFor(secret.getBytes(StandardCharsets.UTF_8));
	}

	private Claims parseClaims(String token) {
		return Jwts.parser()
			.verifyWith((SecretKey) getSigningKey())
			.build()
			.parseSignedClaims(token)
			.getPayload();
	}
}
