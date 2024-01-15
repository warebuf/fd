console.log("loaded js");

const c = document.getElementById("myCanvas");
const ctx = c.getContext("2d");

ctx.beginPath();
ctx.rect(20, 20, 150, 100);
ctx.stroke();
