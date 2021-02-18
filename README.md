# GPU Resource Toleration Admission Controller


## Webhook for Validating Admission Controller
- If with no GPU Resource Request, Toleration of GPU Resource name can not be added into Pod Spec


## Webhook for Mutating Admission Controller
It automatically adds toleration for taint list arguments with NoSchedule and NoExecute operation.


## How to Add Taint to Node
Run `kubectl taint nodes` command like below.

    ```
    kubectl taint nodes {Node Name} {Resource Name}=:NoSchedule
    kubectl taint nodes {Node Name} {Resource Name}=:NoSchedule
    ```


## Host to build Docker Image

```
git clone https://github.com/tmax-cloud/gpu-resource-toleration-admission-controller.git
docker build -t mytag -f Dockerfile .
```
