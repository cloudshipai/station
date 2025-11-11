import React, { useState, useRef, useCallback, useEffect } from 'react';
import { Play, Pause, SkipBack, SkipForward, Gauge } from 'lucide-react';

interface TimelineScrubberProps {
  totalDuration: number; // microseconds
  currentTime: number; // microseconds
  onTimeChange: (time: number) => void;
  isPlaying: boolean;
  onPlayPause: () => void;
  playbackSpeed: number;
  onSpeedChange: (speed: number) => void;
}

export const TimelineScrubber: React.FC<TimelineScrubberProps> = ({
  totalDuration,
  currentTime,
  onTimeChange,
  isPlaying,
  onPlayPause,
  playbackSpeed,
  onSpeedChange,
}) => {
  const [isDragging, setIsDragging] = useState(false);
  const scrubberRef = useRef<HTMLDivElement>(null);

  const formatTime = (micros: number): string => {
    const ms = micros / 1000;
    if (ms < 1000) return `${Math.round(ms)}ms`;
    const seconds = ms / 1000;
    if (seconds < 60) return `${seconds.toFixed(1)}s`;
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  const handleMouseDown = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    setIsDragging(true);
    updateTimeFromMouse(e);
  }, []);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (isDragging) {
      updateTimeFromMouse(e);
    }
  }, [isDragging]);

  const handleMouseUp = useCallback(() => {
    setIsDragging(false);
  }, []);

  const updateTimeFromMouse = (e: MouseEvent | React.MouseEvent) => {
    if (!scrubberRef.current) return;
    
    const rect = scrubberRef.current.getBoundingClientRect();
    const x = Math.max(0, Math.min(e.clientX - rect.left, rect.width));
    const percentage = x / rect.width;
    const newTime = percentage * totalDuration;
    
    onTimeChange(Math.max(0, Math.min(newTime, totalDuration)));
  };

  useEffect(() => {
    if (isDragging) {
      window.addEventListener('mousemove', handleMouseMove);
      window.addEventListener('mouseup', handleMouseUp);
      
      return () => {
        window.removeEventListener('mousemove', handleMouseMove);
        window.removeEventListener('mouseup', handleMouseUp);
      };
    }
  }, [isDragging, handleMouseMove, handleMouseUp]);

  const progress = totalDuration > 0 ? (currentTime / totalDuration) * 100 : 0;

  const speedOptions = [0.5, 1, 2, 4];

  const handleSkipBack = () => {
    onTimeChange(Math.max(0, currentTime - 5000000)); // Skip back 5s
  };

  const handleSkipForward = () => {
    onTimeChange(Math.min(totalDuration, currentTime + 5000000)); // Skip forward 5s
  };

  return (
    <div className="bg-gray-900 border-t border-gray-700 p-4">
      <div className="max-w-7xl mx-auto">
        {/* Controls Row */}
        <div className="flex items-center gap-4 mb-3">
          {/* Playback Controls */}
          <div className="flex items-center gap-2">
            <button
              onClick={handleSkipBack}
              className="p-2 hover:bg-gray-800 rounded transition-colors text-gray-400 hover:text-white"
              title="Skip back 5s"
            >
              <SkipBack className="w-4 h-4" />
            </button>
            
            <button
              onClick={onPlayPause}
              className={`p-3 rounded-lg transition-all ${
                isPlaying
                  ? 'bg-cyan-900/40 border-2 border-cyan-500 text-cyan-300 hover:bg-cyan-900/60'
                  : 'bg-gray-800 border-2 border-gray-600 text-gray-300 hover:bg-gray-700'
              }`}
              title={isPlaying ? 'Pause' : 'Play'}
            >
              {isPlaying ? (
                <Pause className="w-5 h-5" />
              ) : (
                <Play className="w-5 h-5" />
              )}
            </button>

            <button
              onClick={handleSkipForward}
              className="p-2 hover:bg-gray-800 rounded transition-colors text-gray-400 hover:text-white"
              title="Skip forward 5s"
            >
              <SkipForward className="w-4 h-4" />
            </button>
          </div>

          {/* Time Display */}
          <div className="flex items-center gap-2 font-mono text-sm">
            <span className="text-cyan-400 font-semibold">{formatTime(currentTime)}</span>
            <span className="text-gray-500">/</span>
            <span className="text-gray-400">{formatTime(totalDuration)}</span>
          </div>

          <div className="flex-1" />

          {/* Speed Control */}
          <div className="flex items-center gap-2">
            <Gauge className="w-4 h-4 text-gray-400" />
            <div className="flex items-center gap-1">
              {speedOptions.map(speed => (
                <button
                  key={speed}
                  onClick={() => onSpeedChange(speed)}
                  className={`px-3 py-1 rounded font-mono text-xs transition-all ${
                    playbackSpeed === speed
                      ? 'bg-purple-900/40 border border-purple-500 text-purple-300'
                      : 'bg-gray-800 border border-gray-600 text-gray-400 hover:bg-gray-700'
                  }`}
                >
                  {speed}×
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Scrubber Timeline */}
        <div className="relative">
          {/* Track */}
          <div
            ref={scrubberRef}
            className="relative h-8 bg-gray-800 rounded-lg cursor-pointer border border-gray-700 hover:border-gray-600 transition-colors"
            onMouseDown={handleMouseDown}
          >
            {/* Progress Fill */}
            <div
              className="absolute inset-y-0 left-0 bg-gradient-to-r from-cyan-600 to-purple-600 rounded-lg transition-all pointer-events-none"
              style={{ width: `${progress}%` }}
            />

            {/* Playhead Handle */}
            <div
              className="absolute top-1/2 -translate-y-1/2 w-4 h-full bg-white rounded-full shadow-lg border-2 border-cyan-400 pointer-events-none"
              style={{ left: `calc(${progress}% - 8px)` }}
            >
              {/* Playhead Line */}
              <div className="absolute left-1/2 -translate-x-1/2 w-0.5 h-full bg-cyan-400" />
            </div>

            {/* Time Ticks */}
            <div className="absolute inset-0 flex items-center justify-between px-2 pointer-events-none">
              {[0, 0.25, 0.5, 0.75, 1].map((tick, idx) => (
                <div key={idx} className="flex flex-col items-center">
                  <div className="w-px h-2 bg-gray-600" />
                  <span className="text-xs text-gray-500 font-mono mt-1">
                    {formatTime(totalDuration * tick)}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Current Time Indicator (Tooltip) */}
          {isDragging && (
            <div
              className="absolute -top-10 bg-gray-900 border border-cyan-500 rounded px-2 py-1 pointer-events-none"
              style={{ left: `calc(${progress}% - 30px)` }}
            >
              <div className="text-xs font-mono text-cyan-400 whitespace-nowrap">
                {formatTime(currentTime)}
              </div>
              <div className="absolute left-1/2 -translate-x-1/2 bottom-0 translate-y-full w-0 h-0 border-l-4 border-r-4 border-t-4 border-transparent border-t-cyan-500" />
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
