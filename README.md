<div align="center">

# 🤖 Fleks · Altinnator

**Kubernetes operator for managing Kartverket's resources in Altinn's Resource Registry.**

<br/>

[![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-326CE5?style=flat-square&logo=kubernetes&logoColor=white)](https://kubernetes.io)
[![Kubebuilder](https://img.shields.io/badge/Kubebuilder-326CE5?style=flat-square)](https://book.kubebuilder.io)
[![Altinn 3](https://img.shields.io/badge/Altinn-3-1E2B3C?style=flat-square)](https://docs.altinn.studio)
[![Maskinporten](https://img.shields.io/badge/Maskinporten-0069B4?style=flat-square)](https://docs.digdir.no/docs/Maskinporten/)
[![Status](https://img.shields.io/badge/status-in%20development-orange?style=flat-square)](#-about-the-project)

</div>

---

## 📖 Contents

- [🎯 About the project](#-about-the-project)
- [🚀 Getting started](#-getting-started)
- [🔗 Useful links](#-useful-links)

---

## 🎯 About the project

An API that Kartverket offers through Maskinporten can be delegated in Altinn when Altinn's Resource Registry has a
resource of type `MaskinportenSchema` for it. Other access in Altinn is described with a `GenericAccessResource`.
With Altinnator, a team describes such a resource in YAML, and Altinnator makes Altinn match it.

It will handle two kinds, both in `altinnator.kartverket.no/v1alpha1`:

| Kind                         | What it is                                          |
| :--------------------------- | :-------------------------------------------------- |
| `MaskinportenSchemaResource` | A resource that gives access to Maskinporten scopes |
| `GenericAccessResource`      | A resource for other access in Altinn               |

Altinnator is under development, and neither kind is in place yet.

---

## 🚀 Getting started

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to run and test Altinnator locally.

---

## 🔗 Useful links

| Resource                         | Link                                                                             |
| :------------------------------- | :------------------------------------------------------------------------------- |
| Resource Registry API            | https://docs.altinn.studio/nb/api/resourceregistry/                              |
| Resource Registry source code    | https://github.com/Altinn/altinn-auth/tree/main/src/apps/Altinn.ResourceRegistry |
| Kartverket's resources in Altinn | https://tjenesteoversikten.no/org/kv                                             |

---

<div align="center">
<sub>Made by <b>Fleks-Team Tilgangsstyring</b> at Bekk, for Kartverket</sub>
</div>
