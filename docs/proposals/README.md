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

Before starting your work and creating a GEP, please take some time to familiarize yourself with our
general [Gardener Contribution Guidelines](https://gardener.cloud/docs/contribute/).

It is recommended to discuss and outline the motivation of your prospective GEP as a draft with the community before you take the investment of creating the actual GEP. This early briefing supports the understanding for the broad community and leads to a fast feedback for your proposal from the respective experts in the community.
An appropriate format for this may be the regular [Gardener community meetings](https://gardener.cloud/docs/contribute/#bi-weekly-meetings).

## How to file a GEP

GEPs should be created as Markdown `.md` files and are submitted through a GitHub pull request to their current home in [docs/proposals](https://github.com/gardener/gardener/tree/master/docs/proposals). Please use the provided [template](./00-template.md) or follow the structure of existing [GEPs]( https://github.com/gardener/gardener/tree/master/docs/proposals) which makes reviewing easier and faster. Additionally, please link the new GEP in our documentation [index](../README.md#Proposals).

If not already done, please present your GEP in the [regular community meetings](https://gardener.cloud/docs/contribute/#bi-weekly-meetings) to brief the community about your proposal (we strive for personal communication :) ). Also consider that this may be an important step to raise awareness and understanding for everyone involved.

The GEP template contains a small set of metadata, which is helpful for keeping track of the enhancement
in general and especially of who is responsible for implementing and reviewing PRs that are part of
the enhancement.

### Main Reviewers

Apart from general metadata, the GEP should name at least one "main reviewer".
You can find a main reviewer for your GEP either when discussing the proposal in the community meeting, by asking in our
[Slack Channel](https://gardener.cloud/docs/contribute/#slack-channel) or at latest during the GEP PR review.
New GEPs should only be accepted once at least one main reviewer is nominated/assigned. 

The main reviewers are charged with the following tasks:

- familiarizing themselves with the details of the proposal
- reviewing the GEP PR itself and any further updates to the document
- discussing design details and clarifying implementation questions with the author before and after
 the proposal was accepted
- reviewing PRs related to the GEP in-depth

Other community members are of course also welcome to help the GEP author, review his work and raise
general concerns with the enhancement. Nevertheless, the main reviewers are supposed to focus on more
in-depth reviews and accompaning the whole GEP process end-to-end, which helps with getting more
high-quality reviews and faster feedback cycles instead of having more people looking at the process
with lower priority and less focus.

## GEP Process

1. Pre-discussions about GEP (if necessary)
1. Find a main reviewer for your enhancement
1. GEP is filed through GitHub PR
1. Presentation in Gardener community meeting (if possible)
1. Review of GEP from maintainers/community
1. GEP is merged if accepted
1. Implementation of GEP
1. Consider keeping GEP up-to-date in case implementation differs essentially
