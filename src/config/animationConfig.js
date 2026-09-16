export const animationConfig = {
  trainerFrameDuration: 120,
  projectileLaunchDelay: 70,
  projectileDuration: 620,
  projectileArcHeight: { min: 34, max: 102 },
  hitDuration: 260,
};

export const BattleAnimationState = Object.freeze({
  IDLE: "idle",
  THROWING: "throwing",
  PROJECTILE_FLYING: "projectileFlying",
  HIT: "hit",
});