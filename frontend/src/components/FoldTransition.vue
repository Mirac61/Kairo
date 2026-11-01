<script setup lang="ts">
// Auf- und Zuklappen mit echter Höhe: v-if/v-show am Kind schaltet, hier gleitet die Höhe von 0 auf den Inhalt und zurück.
// Dauer und Kurve stehen in design.css (.fold-*); bei „Bewegung reduzieren“ ist die Dauer 0 und es springt wie vorher.
const box = (el: Element) => el as HTMLElement

function grow(el: Element) {
  box(el).style.height = '0'
  void box(el).offsetHeight // Startwert festschreiben, sonst gibt es nichts zu animieren
  box(el).style.height = `${el.scrollHeight}px`
}
function shrink(el: Element) {
  box(el).style.height = `${box(el).offsetHeight}px`
  void box(el).offsetHeight
  box(el).style.height = '0'
}
function reset(el: Element) {
  box(el).style.height = ''
}
</script>

<template>
  <Transition name="fold" @enter="grow" @after-enter="reset" @leave="shrink" @after-leave="reset">
    <slot />
  </Transition>
</template>
