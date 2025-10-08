# Orbit Core

**Orbit Core** is the central repository for the modular monolith backend of the Orbit project, built with **Golang (Go)**. It houses the core business logic services that rely on a strongly relational data model, powered by **PostgreSQL**. This repository is designed to support rapid prototyping and streamlined deployment during the early stages of development, while maintaining a modular structure to facilitate a smooth evolution into a mature microservices architecture.

## Overview

The `orbit-core` repository encapsulates the following key services, implemented as isolated modules within the monolith:

1. **Gateway Service**
   - Acts as the **single entry point** for all client requests.
   - Handles routing of traffic to internal services and enforces rate limiting using **Redis**.

2. **Authentication Service**
   - Manages **secure user verification** and session handling.
   - Implements **JWTs (JSON Web Tokens)** for authentication and uses **Argon2id** for secure credential hashing, with data stored in **PostgreSQL**.

3. **Calendar & Task Service**
   - Provides **core scheduling functionality** and event management.
   - Persists all task and event data in **PostgreSQL**.

4. **Location Service**
   - Manages location-based features, including location tracking and geolocation functionality.

5. **Integration Service**
   - Handles integration with external APIs and data synchronization.

## Core Principles

- **Modularity**: Each service is implemented as a distinct module to ensure clear separation of concerns, enabling easier refactoring into microservices when needed.
- **Scalability**: Designed to support rapid prototyping while laying the foundation for future scalability.
- **Data Integrity**: Relies on **PostgreSQL** as the primary relational database to ensure robust data consistency.

## Related Repositories

To maintain clear boundaries and accommodate different technology stacks and deployment requirements, the following components are maintained in separate repositories:

1. **AI/ML Modules (Python Backend)**
   - Contains services like the **Intelligence Service** for tasks such as natural language parsing (DistilBERT), recommendation generation (TFRS), and LLM inference (vLLM).
   - Isolated due to its **Python-based** tech stack and deployment on **GPU-accelerated nodes** (e.g., HKUST Academic Cloud).

2. **Deployment Repository**
   - Stores high-level deployment configurations, including **Kubernetes** setups and **CI/CD pipelines**.

## Getting Started

### Prerequisites
- **Go**: Version 1.21 or higher
- **PostgreSQL**: Version 15 or higher
- **Redis**: For rate limiting in the Gateway Service
- **Docker**: For local development and testing

### Installation
1. Clone the repository:
   ```bash
   git clone https://github.com/your-org/orbit-core.git
