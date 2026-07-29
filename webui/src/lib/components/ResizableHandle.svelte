<script>
  let { direction = 'horizontal', onResize } = $props();

  let isDragging = $state(false);
  let startPos = 0;

  function handleMouseDown(event) {
    event.preventDefault();
    isDragging = true;
    startPos = direction === 'horizontal' ? event.clientX : event.clientY;

    const handleMouseMove = (moveEvent) => {
      if (!isDragging) return;
      const currentPos = direction === 'horizontal' ? moveEvent.clientX : moveEvent.clientY;
      const delta = currentPos - startPos;
      startPos = currentPos;
      onResize?.(delta);
    };

    const handleMouseUp = () => {
      isDragging = false;
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
    };

    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  role="separator"
  aria-orientation={direction}
  class={[
    direction === 'horizontal' ? 'xsql-resizer-h' : 'xsql-resizer-v',
    isDragging && 'bg-[var(--accent-soft)]'
  ]}
  onmousedown={handleMouseDown}
></div>
