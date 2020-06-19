# Gardener Enhancement Proposal (GEP)

Changes to the Gardener code base are often incorporated directly via pull requests which either themselves contain a description about the motivation and scope of a change or a linked GitHub issue does.

If a perspective feature has a bigger extent, requires the involvement of several parties or more discussion is needed before the actual implementation can be started, you may consider filing a pull request with a Gardener Enhancement Proposal (GEP) first.

GEPs are a measure to propose a change or to add a feature to Gardener, help you to describe the change(s) conceptionally, and to list the steps that are necessary to reach this goal. It helps the Gardener maintainers as well as the community to understand the motivation and scope around your proposed change(s) and encourages their contribution to discussions and future pull requests. If you are familiar with the Kubernetes community, GEPs are analogue to Kubernetes Enhancement Proposals ([KEPs]( https://github.com/kubernetes/enhancements/tree/master/keps)).

## Reasons for a GEP

You may consider filing a GEP for the following reasons:
-	A Gardener architectural change is intended / necessary
-	Major changes to the Gardener code base
-	A phased implementation approach is expected because of the widespread scope of the change
-	Your proposed changes may be controversial

We encourage you to take a look at already merged [GEPs]( https://github.com/gardener/gardener/tree/master/docs/proposals) since they give you a sense of what a typical GEP comprises.

## Before creating a GEP

It is recommended to discuss and outline the motivation of your prospective GEP as a draft with the community before you take the investment of creating the actual GEP. This early briefing supports the understanding for the broad community and leads to a fast feedback for your proposal from the respective experts in the community.
An appropriate format for this may be the regular [Gardener community meeting](https://github.com/gardener/documentation/blob/master/CONTRIBUTING.md#weekly-meeting).

## How to file a GEP

GEPs should be created as Markdown `.md` files and are submitted through a GitHub pull request to their current home in [docs/proposals](https://github.com/gardener/gardener/tree/master/docs/proposals). Please use the provided [template](./00-template.md) or follow the structure of existing [GEPs]( https://github.com/gardener/gardener/tree/master/docs/proposals) which makes reviewing easier and faster. Additionally, please link the new GEP in our documentation [index](../README.md#Proposals).

If not already done, please present your GEP in the [regular community meeting](https://github.com/gardener/documentation/blob/master/CONTRIBUTING.md#weekly-meeting) to brief the community about your proposal (we strive for personal communication :) ). Also consider that this may be an important step to raise awareness and understanding for everyone involved.

## GEP Process

1. Pre-discussions about GEP (if necessary)
2. GEP is filed through GitHub PR
3. Presentation in Gardener community meeting (if possible)
4. Review of GEP from maintainers/community
5. GEP is merged if accepted
6. Implementation of GEP
7. Consider keeping GEP up-to-date in case implementation differs essentially
