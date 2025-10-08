# Agent Service - Azure Deployment Guide

## Overview

This guide provides step-by-step instructions for deploying the agent service to Microsoft Azure using various Azure services. Choose the deployment method that best fits your requirements.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Deployment Options](#deployment-options)
3. [Option 1: Azure Container Instances (Quickest)](#option-1-azure-container-instances-quickest)
4. [Option 2: Azure Kubernetes Service (Production)](#option-2-azure-kubernetes-service-production)
5. [Option 3: Azure App Service (Managed)](#option-3-azure-app-service-managed)
6. [Option 4: Azure Virtual Machine (Full Control)](#option-4-azure-virtual-machine-full-control)
7. [Azure Configuration](#azure-configuration)
8. [Monitoring & Logging](#monitoring--logging)
9. [Cost Estimation](#cost-estimation)
10. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Tools

```bash
# Install Azure CLI
curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash

# Or on macOS
brew install azure-cli

# Or on Windows
# Download from: https://aka.ms/installazurecliwindows

# Verify installation
az --version
```

### Required Accounts

- Azure Subscription (free tier available)
- Azure Container Registry (ACR) for Docker images
- GitHub account (for CI/CD integration)

### Login to Azure

```bash
# Login to Azure
az login

# Set your subscription
az account set --subscription "YOUR_SUBSCRIPTION_ID"

# Verify
az account show
```

---

## Deployment Options

| Option | Best For | Cost | Setup Time | Scalability |
|--------|----------|------|------------|-------------|
| **Azure Container Instances** | Quick testing, demos | $ | 5 min | Low |
