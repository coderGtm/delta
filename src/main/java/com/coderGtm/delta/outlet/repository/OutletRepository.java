package com.coderGtm.delta.outlet.repository;

import java.util.Optional;
import java.util.UUID;

import org.springframework.data.jpa.repository.JpaRepository;

import com.coderGtm.delta.outlet.entity.Outlet;

/**
 * Repository for persisting and querying outlet records.
 */
public interface OutletRepository extends JpaRepository<Outlet, UUID> {

	Optional<Outlet> findByIdAndRemovedAtIsNull(UUID id);
}
