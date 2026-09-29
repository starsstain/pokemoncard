import backTallGrass from "../assets/back_tall_grass.png";
import groundTallGrass from "../assets/ground_tall_grass.png";
import eeveeBack from "../assets/eevee_back.png";
import bulbasaurFront from "../assets/bulbasaur_front.png";
import dialogBox from "../assets/dialog_box.png";
import attacked from "../assets/attacked.png";
import trainerFrame1 from "../assets/player/brendan_playerback_1.png";
import trainerFrame2 from "../assets/player/brendan_playerback_2.png";
import trainerFrame3 from "../assets/player/brendan_playerback_3.png";
import trainerFrame4 from "../assets/player/brendan_playerback_4.png";

export const battleAssets = {
  backTallGrass,
  groundTallGrass,
  eeveeBack,
  bulbasaurFront,
  dialogBox,
  attacked,
  trainerFrames: [trainerFrame1, trainerFrame2, trainerFrame3, trainerFrame4],
};

// Vite builds this list automatically, so adding a PNG in assets/icons needs no code change.
const iconModules = import.meta.glob("../assets/icons/*.png", {
  eager: true,
  import: "default",
});

export const projectileIcons = Object.values(iconModules);
