// Hand-rolled critically-damped spring animator, no dependency. Reads and
// updates a single live numeric value via requestAnimationFrame rather than
// a fixed-duration transition, so a spring already in flight can be
// re-targeted (e.g. a modal opening then immediately closing) and it
// continues from its current on-screen value and velocity instead of
// jumping -- interruptibility is the whole point, not an edge case.
//
// Damping/response are the designer-friendly parameters from Apple's HIG
// motion guidance: damping 1.0 is critically damped (no overshoot),
// response is the approximate settle time in seconds.

export interface SpringOptions {
  damping?: number;
  response?: number;
  onUpdate: (value: number) => void;
  onComplete?: () => void;
}

export interface SpringHandle {
  set: (target: number, velocity?: number) => void;
  stop: () => void;
}

const REST_EPSILON = 0.001;

export function createSpring(initial: number, opts: SpringOptions): SpringHandle {
  let value = initial;
  let velocity = 0;
  let target = initial;
  let frame: number | null = null;

  const damping = opts.damping ?? 1;
  const response = opts.response ?? 0.35;
  const angularFreq = (2 * Math.PI) / response;
  const stiffness = angularFreq * angularFreq;
  const dampingCoeff = 2 * damping * angularFreq;

  function step(dt: number) {
    const force = -stiffness * (value - target) - dampingCoeff * velocity;
    velocity += force * dt;
    value += velocity * dt;
  }

  function loop(now: number, last: number) {
    const dt = Math.min((now - last) / 1000, 1 / 30);
    step(dt);

    if (Math.abs(value - target) < REST_EPSILON && Math.abs(velocity) < REST_EPSILON) {
      value = target;
      velocity = 0;
      opts.onUpdate(value);
      frame = null;
      opts.onComplete?.();
      return;
    }

    opts.onUpdate(value);
    frame = requestAnimationFrame((t) => loop(t, now));
  }

  return {
    set(newTarget, newVelocity) {
      target = newTarget;
      if (newVelocity !== undefined) velocity = newVelocity;
      if (frame == null) {
        frame = requestAnimationFrame((t) => loop(t, t));
      }
    },
    stop() {
      if (frame != null) cancelAnimationFrame(frame);
      frame = null;
    },
  };
}

export function prefersReducedMotion(): boolean {
  return typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
