-- Office geographic coordinates (for the Peta Lokasi / office-map screen).
-- double precision (not numeric): sqlc maps numeric -> Go string; float8 -> *float64,
-- which serializes as a JSON number for the map client.
ALTER TABLE masterdata.offices
  ADD COLUMN latitude  double precision,
  ADD COLUMN longitude double precision;
