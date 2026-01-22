const ifl_btn = document.querySelector("#install-for-linux");
const iifl_d = document.querySelector("#install-instructions-for-linux");
async function AddClass(element, className) {
  element.classList.add(className);
}

async function Wait(time) {
  await new Promise((resolve) => setTimeout(resolve, time));
}

async function RemoveClass(element, className) {
  element.classList.remove(className);
}

ifl_btn.addEventListener("click", async () => {
  await AddClass(iifl_d, "appear-animation");
  await Wait(1500);
  await RemoveClass(iifl_d, "appear-animation");
  await AddClass(iifl_d, "animation-done");
});
