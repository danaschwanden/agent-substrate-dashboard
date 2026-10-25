# Fail visibly on frontend and backend version skew

The Dashboard Frontend will verify the backend’s advertised API version before rendering live data. Unsupported versions will produce a dedicated incompatibility screen that reports both component versions instead of attempting partial or best-effort rendering.
