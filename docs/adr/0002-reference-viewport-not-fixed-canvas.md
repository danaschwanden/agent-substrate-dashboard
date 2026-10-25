# Treat 1280×1080 as a reference viewport, not a fixed canvas

The initial Lit rewrite will use 1280×1080 screenshot tests to preserve the demonstrated composition, while avoiding fixed page geometry or assumptions that cards always occupy their current columns. Components and layout contracts must remain amenable to additional viewport tests and dynamic rearrangement without redesigning their data or styling boundaries.
