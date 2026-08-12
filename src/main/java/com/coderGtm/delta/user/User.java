package com.coderGtm.delta.user;

import java.time.Instant;
import java.util.UUID;

import com.coderGtm.delta.common.entity.BaseEntity;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * Local application user synchronized from the external authentication system.
 *
 * <p>The {@code authUid} links this record to the upstream Firebase identity,
 * while {@code deletedAt} allows soft deletion without immediately removing
 * related business records.</p>
 */
@Entity
@Table(name = "users")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class User extends BaseEntity {

	@Id
	@GeneratedValue(strategy = GenerationType.UUID)
	private UUID id;

	@Column(unique = true)
	private String authUid;

	@Column(nullable = false)
	private String name;

	@Column(unique = true)
	private String email;

	private String phone;

	private Instant deletedAt;

	/**
	 * Preserves the last known email after account deletion so the unique email
	 * constraint stays satisfied while the active email column is freed up for a
	 * future account that reuses the same provider email.
	 */
	private String historicalEmail;
}
