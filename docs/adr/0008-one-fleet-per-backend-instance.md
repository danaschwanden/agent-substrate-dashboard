# Observe one fleet per backend instance

Each Dashboard Backend instance will observe one configured atespace and one selected worker pool. This keeps authorization narrow and fleet statistics coherent; installations that need more scopes can deploy more instances, while selector-based configuration leaves room for future aggregation without changing the frontend’s data model.
