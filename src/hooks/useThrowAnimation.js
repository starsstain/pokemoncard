import { useCallback, useEffect, useRef, useState } from "react";
import { animationConfig, BattleAnimationState } from "../config/animationConfig";
import { projectileIcons } from "../utils/assetLoader";
import { getRandomInteger, getRandomItem } from "../utils/random";

const initialAnimation = {
  phase: BattleAnimationState.IDLE,
  trainerFrame: 1,
  projectileIcon: null,
  projectileArcHeight: null,
  enemyIsHit: false,
};

const wait = (duration) => new Promise((resolve) => window.setTimeout(resolve, duration));

export function useThrowAnimation() {
  const [animation, setAnimation] = useState(initialAnimation);
  const animationRef = useRef(initialAnimation);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);

  const updateAnimation = useCallback((next) => {
    animationRef.current = next;
    setAnimation(next);
  }, []);

  const startThrow = useCallback(async () => {
    if (animationRef.current.phase !== BattleAnimationState.IDLE) return;
    updateAnimation({ ...initialAnimation, phase: BattleAnimationState.THROWING });

    for (const trainerFrame of [2, 3, 4]) {
      await wait(animationConfig.trainerFrameDuration);
      if (!mountedRef.current) return;
      updateAnimation({ ...animationRef.current, trainerFrame });
    }

    await wait(animationConfig.projectileLaunchDelay);
    if (!mountedRef.current) return;
    updateAnimation({
      ...animationRef.current,
      phase: BattleAnimationState.PROJECTILE_FLYING,
      projectileIcon: getRandomItem(projectileIcons),
      projectileArcHeight: getRandomInteger(
        animationConfig.projectileArcHeight.min,
        animationConfig.projectileArcHeight.max,
      ),
    });
  }, [updateAnimation]);

  const completeProjectile = useCallback(async () => {
    if (animationRef.current.phase !== BattleAnimationState.PROJECTILE_FLYING) return;
    updateAnimation({
      ...animationRef.current,
      phase: BattleAnimationState.HIT,
      projectileIcon: null,
      enemyIsHit: true,
    });

    await wait(animationConfig.hitDuration);
    if (!mountedRef.current) return;
    updateAnimation(initialAnimation);
  }, [updateAnimation]);

  return {
    ...animation,
    isAnimating: animation.phase !== BattleAnimationState.IDLE,
    startThrow,
    completeProjectile,
  };
}