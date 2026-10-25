# Model multiple actors per worker

The backend and Worker Map will represent each worker’s assignment set and resource allocation without assuming one actor per worker. Summary occupancy can count workers with assignments, but joins, labels, and capacity displays must preserve Agent Substrate’s zero-to-many worker relationship.
