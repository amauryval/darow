<script setup lang="ts">
import { computed } from "vue";
import { Circle, Magnet, MapPin, Move, Pentagon, Share2, Spline, Undo2 } from "@lucide/vue";
import type { DrawShape } from "../../composables/useMapDraw";
import { EDIT_LEGEND } from "../../utils/drawStyles";
import Badge from "../Badge.vue";
import BaseButton from "../ui/BaseButton.vue";
import ButtonGroup from "../ui/ButtonGroup.vue";
import Typography from "../ui/Typography.vue";

// The edit session's bar, overlaid on the map for as long as the mode lasts.
// Presentational: it renders the session's state and emits intentions, and
// knows nothing of the layer, the network or what a save writes (§0.5).
const props = defineProps<{
  loadingFeatures?: boolean;
  saving?: boolean;
  shape: DrawShape | null;
  // Placing: a vertex put down near a neighbour is pulled onto it.
  snapping: boolean;
  // Moving: whether a coordinate two geometries already share is one vertex
  // or two.
  topological: boolean;
  // Moving: the drag grabs the whole geometry under the pointer instead of
  // one of its vertices. The two cannot share a drag, so it is a mode.
  moving: boolean;
  cuttingHole: boolean;
  // Whether a shape is being drawn at all — what tells the bar that taking
  // back a point means something.
  drawing: boolean;
  // Whether anything in view can hold a hole; a hole has no meaning on a
  // layer of lines or points.
  hasAreal: boolean;
  changedCount: number;
  createdCount: number;
}>();

const emit = defineEmits<{
  pick: [shape: DrawShape];
  toggleHole: [];
  toggleMoving: [];
  toggleSnapping: [];
  toggleTopology: [];
  undo: [];
  save: [];
  cancel: [];
}>();

const dirtyCount = computed(() => props.changedCount + props.createdCount);

// Creating and modifying are two different intentions on the same layer, and
// a single undifferentiated row of buttons made *Trou* read as a fourth shape
// to draw rather than as a cut into a shape already there.
const shapes: Array<{ value: DrawShape; label: string; icon: typeof Spline }> = [
  { value: "Polygon", label: "Polygone", icon: Pentagon },
  { value: "LineString", label: "Ligne", icon: Spline },
  { value: "Point", label: "Point", icon: MapPin },
];
</script>

<template>
  <div class="flex flex-col gap-2">
    <div class="flex flex-wrap items-center gap-8">
      <div class="flex items-center gap-1.5">
        <Typography variant="eyebrow">Créer</Typography>
        <ButtonGroup>
          <BaseButton
            v-for="s in shapes"
            :key="s.value"
            joined
            size="sm"
            :variant="shape === s.value ? 'primary' : 'secondary'"
            :icon="s.icon"
            :aria-pressed="shape === s.value"
            :title="`Dessiner ${s.label.toLowerCase()} — une nouvelle ligne dans la table`"
            @click="emit('pick', s.value)"
          />
        </ButtonGroup>
      </div>

      <div class="flex items-center gap-1.5">
        <Typography variant="eyebrow">Modifier</Typography>
        <ButtonGroup>
          <!-- A tool that cannot apply says so rather than answering nothing
                (§8.2). -->
          <BaseButton
            joined
            size="sm"
            :variant="cuttingHole ? 'primary' : 'secondary'"
            :icon="Circle"
            :aria-pressed="cuttingHole"
            :disabled="!hasAreal"
            :title="
              hasAreal
                ? 'Découper un trou dans le polygone qui le contient'
                : 'Aucun polygone dans la session : survolez-en un pour le rendre modifiable'
            "
            @click="emit('toggleHole')"
          />
          <BaseButton
            joined
            size="sm"
            :variant="moving ? 'primary' : 'secondary'"
            :icon="Move"
            :aria-pressed="moving"
            title="Déplacer la géométrie entière survolée, au lieu d’un seul de ses sommets"
            @click="emit('toggleMoving')"
          />
          <BaseButton
            joined
            size="sm"
            :icon="Undo2"
            :disabled="!drawing"
            :title="drawing ? 'Retirer le dernier point posé' : 'Aucune forme en cours de dessin'"
            @click="emit('undo')"
          />
        </ButtonGroup>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <Typography variant="eyebrow">Outils</Typography>
        <BaseButton
          size="sm"
          :variant="snapping ? 'primary' : 'secondary'"
          :icon="Magnet"
          :aria-pressed="snapping"
          title="Aimanter un sommet posé sur les sommets et segments voisins"
          @click="emit('toggleSnapping')"
        />
        <BaseButton
          size="sm"
          :variant="topological ? 'primary' : 'secondary'"
          :icon="Share2"
          :aria-pressed="topological"
          :title="
            topological
              ? 'Déplacer un sommet partagé déplace toutes les géométries qui le portent'
              : 'Seule la géométrie sous le pointeur est déplacée, ses voisines gardent leur sommet'
          "
          @click="emit('toggleTopology')"
        />
      </div>

      <div class="flex gap-2 ml-auto">
        <Badge v-if="changedCount > 0" tone="warning">
          {{ changedCount }} modifiée{{ changedCount > 1 ? "s" : "" }}
        </Badge>
        <Badge v-if="createdCount > 0" tone="primary">
          {{ createdCount }} nouvelle{{ createdCount > 1 ? "s" : "" }}
        </Badge>
      </div>

      <div class="flex flex-col gap-2 ml-auto">
        <div class="flex gap-2 ml-auto">
          <BaseButton size="sm" @click="emit('cancel')">Annuler</BaseButton>
          <BaseButton
            variant="primary"
            size="sm"
            :busy="saving"
            :disabled="saving || dirtyCount === 0"
            :title="dirtyCount === 0 ? 'Rien n’a été modifié' : undefined"
            @click="emit('save')"
          >
            {{ saving ? "Enregistrement…" : "Enregistrer" }}
          </BaseButton>
        </div>

        <ul class="ml-auto flex items-center gap-3">
          <li v-for="entry in EDIT_LEGEND" :key="entry.label" class="flex items-center gap-1">
            <span class="size-2.5 shrink-0 rounded-full" :style="{ backgroundColor: entry.tone }" aria-hidden="true" />
            <Typography variant="caption">{{ entry.label }}</Typography>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>
