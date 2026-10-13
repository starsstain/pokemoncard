import React, { useEffect, useRef } from "react";
import { DEBUG } from "../../config/debug";
import { battleLayout } from "../../config/battleLayout";
import { pokemonStats } from "../../config/pokemonStats";
import { useThrowAnimation } from "../../hooks/useThrowAnimation";
import { defaultDamages, useBattle } from "../../hooks/useBattle";
import { battleAssets } from "../../utils/assetLoader";
import { requestDebugStatus } from "../../services/attackValidation";
import { reportRumAction, reportRumError } from "../../monitoring/rum";
import { GrassPlatform } from "../GrassPlatform/GrassPlatform";
import { HealthBar } from "../HealthBar/HealthBar";
import { Pokemon } from "../Pokemon/Pokemon";
import { Projectile } from "../Projectile/Projectile";
import { Trainer } from "../Trainer/Trainer";
import "./BattleScene.css";

// dev 面板仅用于联调：npm run dev（开发者模式）下显示，npm run prod（生产模式）构建的产物里不包含
const isDevToolsAvailable = import.meta.env.DEV;

export function BattleScene() {
  const animation = useThrowAnimation();
  const [dialoguePhase, setDialoguePhase] = React.useState("intro");
  const [dialogueVersion, setDialogueVersion] = React.useState(0);
  const [dialogueText, setDialogueText] = React.useState("");
  const [isTyping, setIsTyping] = React.useState(true);
  const [isBattleActive, setIsBattleActive] = React.useState(false);
  const dialogueSpeedRef = useRef(1);
  const [serviceMessage, setServiceMessage] = React.useState(null);
  const serviceMessageTimerRef = useRef(null);
  useEffect(() => () => window.clearTimeout(serviceMessageTimerRef.current), []);

  // 格式错误与服务故障（400、5xx、无状态码）只短暂提示，不判定篡改
  const handleServiceDegraded = React.useCallback((status) => {
    const messageByStatus = {
      400: "请求格式错误",
      500: "服务异常",
      502: "网关错误",
      503: "服务暂不可用",
      504: "请求超时",
    };
    setServiceMessage(messageByStatus[status] ?? "网络异常");
    window.clearTimeout(serviceMessageTimerRef.current);
    serviceMessageTimerRef.current = window.setTimeout(() => setServiceMessage(null), 2500);
  }, []);

  const battle = useBattle({ isActive: isBattleActive, onServiceDegraded: handleServiceDegraded });
  const dialogueMessage = dialoguePhase === "intro"
    ? "野生的妙蛙种子出现了！伊布！准备战斗。 (点击继续)"
    : "天哪！他们已经打起来了，快点击鼠标左键帮助伊布。";

  useEffect(() => {
    let characterIndex = 0;
    let timerId;
    dialogueSpeedRef.current = 1;
    setDialogueText("");
    setIsTyping(true);

    const typeNextCharacter = () => {
      characterIndex += 1;
      setDialogueText(dialogueMessage.slice(0, characterIndex));
      if (characterIndex < dialogueMessage.length) {
        timerId = window.setTimeout(typeNextCharacter, 18 / dialogueSpeedRef.current);
        return;
      }

      setIsTyping(false);

    };

    timerId = window.setTimeout(typeNextCharacter, 180);
    return () => window.clearTimeout(timerId);
  }, [dialogueMessage, dialoguePhase, dialogueVersion]);
  const [isDevPanelOpen, setIsDevPanelOpen] = React.useState(false);
  const [devDamageType, setDevDamageType] = React.useState("enemy_collision");
  const [devDamage, setDevDamage] = React.useState("");

  // 投掷物命中时对敌方宝可梦结算伤害（前端独立扣血，校验按抽样异步上报）
  const submitDamageRef = useRef(battle.submitDamage);
  submitDamageRef.current = battle.submitDamage;
  useEffect(() => {
    if (!animation.enemyIsHit) return;
    submitDamageRef.current("enemy", "throw");
  }, [animation.enemyIsHit]);

  const someoneFainted = battle.playerHp <= 0 || battle.enemyHp <= 0;
  const isGameOver = someoneFainted || battle.isTampered;

  // 判定篡改后立即终止对局：停自动攻击、禁投掷，只能重开
  useEffect(() => {
    if (battle.isTampered) setIsBattleActive(false);
  }, [battle.isTampered]);

  // 结算页出现后立即冻结：中断在途的投掷动画、清掉排队中的补扔
  const cancelAnimationRef = useRef(animation.cancelAnimation);
  cancelAnimationRef.current = animation.cancelAnimation;
  useEffect(() => {
    if (isGameOver) cancelAnimationRef.current();
  }, [isGameOver]);

  const handlePointerDown = (event) => {
    if (event.button !== 0) return;
    if (isGameOver) return;
    if (dialoguePhase === "intro") {
      if (isTyping) {
        dialogueSpeedRef.current = 2;
        return;
      }
      setDialoguePhase("battle");
      setIsBattleActive(true);
      return;
    }
    if (!isBattleActive) return; // 分出胜负后禁止投掷，点"再来一局"后恢复
    animation.registerTap();
  };

  const handleDebugStatusClick = (statusCode) => {
    reportRumAction("dev_status_request", { status_code: statusCode });
    const responseStatus = requestDebugStatus(statusCode)
      .then((response) => response.status)
      .catch(() => null);
    // 422 演练完整链路：先看到被篡改的一击生效，稍后才出判决（对局结算后除外）
    if (statusCode === 422 && !isGameOver) {
      battle.stageTamperedAttack(responseStatus);
      return;
    }
    void responseStatus.then((status) => battle.simulateStatus(status));
  };

  const handleRumErrorDemo = () => {
    reportRumAction("dev_frontend_error_demo");
    const error = new Error("Datadog RUM dev 演练：模拟前端 JavaScript 异常");
    reportRumError(error, { source: "dev_panel", error_type: "frontend_demo" });
    setServiceMessage("已将模拟前端异常上报至 RUM（页面继续运行）");
    window.clearTimeout(serviceMessageTimerRef.current);
    serviceMessageTimerRef.current = window.setTimeout(() => setServiceMessage(null), 2500);
  };

  const handleRumResourceErrorDemo = async () => {
    reportRumAction("dev_resource_error_demo");
    try {
      const response = await fetch("http://127.0.0.1:8080/api/v1/rum-monitoring-demo/missing", {
        method: "GET",
      });
      if (!response.ok) {
        reportRumError(new Error(`RUM 演练资源请求失败：HTTP ${response.status}`), {
          source: "dev_panel",
          resource: "backend_demo_endpoint",
          http_status: response.status,
        });
        setServiceMessage(`资源请求失败：HTTP ${response.status}（已记录）`);
      } else {
        setServiceMessage("资源请求返回成功；若要演练错误，请确认后端关闭或返回错误");
      }
    } catch (error) {
      reportRumError(error, { source: "dev_panel", resource: "backend_demo_endpoint" });
      setServiceMessage("资源请求失败：网络异常（已记录）");
    }
    window.clearTimeout(serviceMessageTimerRef.current);
    serviceMessageTimerRef.current = window.setTimeout(() => setServiceMessage(null), 2500);
  };

  // dev：修改某类攻击的名义伤害（扣血与上报同值），用于演练篡改检测；不允许负数，0 可以
  const handleSetDamage = () => {
    if (devDamage === "") return;
    const value = Number(devDamage);
    if (!Number.isFinite(value) || value < 0) return;
    battle.setDamage(devDamageType, Math.round(value));
  };

  const handleResetDamage = () => {
    setDevDamage("");
    battle.resetDamages();
  };

  const resetBattle = () => {
    setDialoguePhase("intro");
    setDialogueVersion((version) => version + 1);
    setIsBattleActive(false);
    battle.resetBattle();
  };

  return (
    <section className="battle-frame" aria-label="Pixel-art battle demo">
      <div
        className={`battle-scene ${animation.isAnimating ? "battle-scene--animating" : ""}`}
        onPointerDown={handlePointerDown}
        role="application"
        aria-label={animation.isAnimating ? "Attack animation in progress" : "Click to throw an item"}
      >
        <img className="battle-background" src={battleAssets.backTallGrass} alt="Tall grass battlefield" draggable="false" />
        <img className="battle-dialog-box" src={battleAssets.dialogBox} alt="" draggable="false" />
        <p className={`battle-dialog-text ${isTyping ? "battle-dialog-text--typing" : ""}`} aria-live="polite">
          {dialogueText}
        </p>

        {serviceMessage && (
          <div className="battle-toast" role="status">{serviceMessage}</div>
        )}

        {isDevToolsAvailable && (
          <div className="dev-tools" onPointerDown={(event) => event.stopPropagation()}>
            <button
              className="dev-tools__toggle"
              type="button"
              onClick={() => setIsDevPanelOpen((isOpen) => !isOpen)}
            >
              dev
            </button>

            {isDevPanelOpen && (
              <div className="dev-tools__panel">
                <span className="dev-tools__label">Datadog RUM 演练：</span>
                <button className="dev-tools__status" type="button" onClick={handleRumErrorDemo}>
                  模拟 JS 异常
                </button>
                <button className="dev-tools__status" type="button" onClick={handleRumResourceErrorDemo}>
                  模拟资源请求失败
                </button>
                <span className="dev-tools__label">返回状态码：</span>
                {[200, 400, 422, 500, 502, 503, 504].map((statusCode) => (
                  <button
                    key={statusCode}
                    className="dev-tools__status"
                    type="button"
                    onClick={() => handleDebugStatusClick(statusCode)}
                  >
                    {statusCode}
                  </button>
                ))}

                <span className="dev-tools__label">攻击力：</span>
                <select
                  className="dev-tools__field"
                  value={devDamageType}
                  onChange={(event) => setDevDamageType(event.target.value)}
                >
                  <option value="enemy_collision">我方撞击</option>
                  <option value="collision">敌方撞击</option>
                  <option value="throw">丢球</option>
                </select>
                <input
                  className="dev-tools__field dev-tools__field--input"
                  type="number"
                  min={0}
                  value={devDamage}
                  placeholder={String(defaultDamages[devDamageType])}
                  onChange={(event) => setDevDamage(event.target.value)}
                />
                <button className="dev-tools__status" type="button" onClick={handleSetDamage}>
                  设置
                </button>
                <button className="dev-tools__status" type="button" onClick={handleResetDamage}>
                  还原
                </button>
              </div>
            )}
          </div>
        )}


        <HealthBar
          name={pokemonStats.enemy.name}
          hp={battle.enemyHp}
          maxHp={pokemonStats.enemy.maxHp}
          attack={battle.damages.collision}
          side="enemy"
        />
        <HealthBar
          name={pokemonStats.player.name}
          hp={battle.playerHp}
          maxHp={pokemonStats.player.maxHp}
          attack={battle.damages.enemy_collision}
          side="player"
        />

        <GrassPlatform side="enemy" layout={battleLayout.enemyPlatform} />
        <Pokemon
          sprite={battleAssets.bulbasaurFront}
          side="enemy"
          isHit={animation.enemyIsHit || battle.enemyIsHit}
          isAttacking={battle.enemyAttacking}
          hitEffect={battleAssets.attacked}
          isFainted={battle.enemyHp <= 0}
          layout={battleLayout.enemyPokemon}
          alt="Bulbasaur"
        />

        <GrassPlatform side="player" layout={battleLayout.playerPlatform} />
        <Trainer frame={animation.trainerFrame} layout={battleLayout.playerTrainer} />
        <Pokemon
          sprite={battleAssets.eeveeBack}
          side="player"
          isHit={battle.playerIsHit}
          isAttacking={battle.playerAttacking}
          hitEffect={battleAssets.attacked}
          isFainted={battle.playerHp <= 0}
          layout={battleLayout.playerPokemon}
          alt="Eevee"
        />

        <Projectile
          sprite={animation.projectileIcon}
          start={battleLayout.projectile.start}
          target={battleLayout.projectile.target}
          arcHeight={animation.projectileArcHeight}
          speedMultiplier={animation.throwSpeed}
          onComplete={animation.completeProjectile}
        />

        {isGameOver && (
          <div className="battle-result">
            <p className={`battle-result__text ${battle.isTampered ? "battle-result__text--tampered" : battle.enemyHp <= 0 ? "battle-result__text--win" : "battle-result__text--lose"}`}>
              {battle.isTampered ? "数据被篡改" : battle.enemyHp <= 0 ? "成功！" : "失败…"}
            </p>
            <button className="battle-restart" onClick={resetBattle}>
              再来一局
            </button>
          </div>
        )}

        {DEBUG && <DebugOverlay />}
      </div>
    </section>
  );
}

function DebugOverlay() {
  const { projectile, enemyPokemon } = battleLayout;
  return (
    <div className="debug-overlay" aria-hidden="true">
      <DebugDot point={projectile.start} label="start" />
      <DebugDot point={projectile.target} label="target" />
      <div className="debug-box" style={{ left: `${enemyPokemon.x / 3.2}%`, top: `${enemyPokemon.y / 2.4}%`, width: `${enemyPokemon.width / 3.2}%`, height: `${enemyPokemon.height / 2.4}%` }} />
    </div>
  );
}

function DebugDot({ point, label }) {
  return <span className="debug-dot" data-label={label} style={{ left: `${point.x / 3.2}%`, top: `${point.y / 2.4}%` }} />;
}
