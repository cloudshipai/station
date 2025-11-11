import { useState, useCallback, useEffect, useRef } from 'react';

interface UsePlaybackOptions {
  totalDuration: number; // microseconds
  onTimeUpdate?: (time: number) => void;
}

export const usePlayback = ({ totalDuration, onTimeUpdate }: UsePlaybackOptions) => {
  const [currentTime, setCurrentTime] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);
  const [playbackSpeed, setPlaybackSpeed] = useState(1);
  const animationFrameRef = useRef<number>();
  const lastUpdateRef = useRef<number>(Date.now());

  const handleTimeChange = useCallback((time: number) => {
    const clampedTime = Math.max(0, Math.min(time, totalDuration));
    setCurrentTime(clampedTime);
    
    if (onTimeUpdate) {
      onTimeUpdate(clampedTime);
    }
  }, [totalDuration, onTimeUpdate]);

  const handlePlayPause = useCallback(() => {
    setIsPlaying(prev => !prev);
  }, []);

  const handleSpeedChange = useCallback((speed: number) => {
    setPlaybackSpeed(speed);
  }, []);

  const reset = useCallback(() => {
    setCurrentTime(0);
    setIsPlaying(false);
    if (onTimeUpdate) {
      onTimeUpdate(0);
    }
  }, [onTimeUpdate]);

  // Animation loop for playback
  useEffect(() => {
    if (!isPlaying) {
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current);
      }
      return;
    }

    const animate = () => {
      const now = Date.now();
      const deltaMs = now - lastUpdateRef.current;
      lastUpdateRef.current = now;

      setCurrentTime(prevTime => {
        // Convert delta milliseconds to microseconds and apply speed
        const deltaMicros = deltaMs * 1000 * playbackSpeed;
        const newTime = prevTime + deltaMicros;

        if (newTime >= totalDuration) {
          setIsPlaying(false);
          if (onTimeUpdate) {
            onTimeUpdate(totalDuration);
          }
          return totalDuration;
        }

        if (onTimeUpdate) {
          onTimeUpdate(newTime);
        }

        return newTime;
      });

      animationFrameRef.current = requestAnimationFrame(animate);
    };

    lastUpdateRef.current = Date.now();
    animationFrameRef.current = requestAnimationFrame(animate);

    return () => {
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current);
      }
    };
  }, [isPlaying, playbackSpeed, totalDuration, onTimeUpdate]);

  return {
    currentTime,
    isPlaying,
    playbackSpeed,
    handleTimeChange,
    handlePlayPause,
    handleSpeedChange,
    reset,
  };
};
