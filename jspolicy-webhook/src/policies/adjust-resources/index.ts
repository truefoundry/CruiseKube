import { adjustResources } from "./resourceUtils";

const [changed, podObject] = adjustResources(request);

if (changed) {
  mutate(podObject);
  print("Successfully applied resource adjustments to pod " + request.namespace + "/" + request.name);
} else {
  print("No significant resource adjustments needed for pod " + request.namespace + "/" + request.name);
}