Overview
========

The Identity Service issues tenant-scoped JSON Web Tokens (JWTs), stores account
metadata with PostgreSQL row-level security, and provides idempotent account
creation endpoints.

Key Features
------------

* **Token issuance** – generates access tokens containing tenant scopes consumed by
  the Activity and Exercise Ontology services.
* **Idempotent account creation** – protects against duplicate submissions by
  storing idempotency keys per tenant.
* **Rate limiting** – safeguards APIs with sliding-window limiters backed by either
  in-memory state or Redis when configured.

Getting Started
---------------

Run the service locally via the project Taskfile:

.. code-block:: bash

   task smoke:compose

The command launches the docker-compose stack, including the identity service and
its dependencies. Refer to the project README for end-to-end workflows.
