<script setup lang="ts">
import { Circle, Magnet, MapPin, Move, Pentagon, Share2, Spline, Undo2 } from "@lucide/vue";
import type { DrawShape } from "../../composables/useMapDraw";
import { FAMILY_LABELS, type GeometryFamily } from "../../utils/geometryFamilies";
import BaseButton from "../ui/BaseButton.vue";
import ButtonGroup from "../ui/ButtonGroup.vue";
import Divider from "../ui/Divider.vue";
import EditSessionActions from "../tables/EditSessionActions.vue";
import Typography from "../ui/Typography.vue";

// The edit session's bar, overlaid on the map for as long as the mode lasts.
// Presentational: it renders the session's state and emits intentions, and
// knows nothing of the layer, the network or what a save writes (§0.5).
const props = defineProps<{
  // The family the table is locked to, null when it takes anything. A table
  // already homogeneous refuses a geometry of another family on write, so the
  // tool that would draw one is not offered (§8.8).
  family: GeometryFamily | null;
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
  // The rows whose cells the session retyped. They are not the map's, but
  // they are the same save: one session, one *Enregistrer* (§8.8).
  rowCount: number;
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

// Creating and modifying are two different intentions on the same layer, and
// a single undifferentiated row of buttons made *Trou* read as a fourth shape
// to draw rather than as a cut into a shape already there.
const shapes: Array<{ value: DrawShape; label: string; family: GeometryFamily; icon: typeof Spline }> = [
  { value: "Polygon", label: "Polygone", family: "polygon", icon: Pentagon },
  { value: "LineString", label: "Ligne", family: "line", icon: Spline },
  { value: "Point", label: "Point", family: "point", icon: MapPin },
];

// Disabled rather than hidden: what a table cannot hold is worth reading once,
// and a bar whose buttons come and go from one table to the next is a bar the
// user has to re-learn each time (§8.2).
function refused(shape: (typeof shapes)[number]): string {
  return props.family === null || props.family === shape.family
    ? ""
    : `Cette table ne contient que des ${FAMILY_LABELS[props.family]} : le serveur refuserait cette géométrie`;
}
</script>

<template>
  <!-- One row, read left to right: what the session can do, then what it has
       done, then how it ends. The registers are told apart by a rule rather
       than by empty space, so the bar keeps its shape when the panel narrows;
       the colours the session draws in are named on the map itself, next to
       what carries them. -->
  <div class="flex min-w-0 flex-1 flex-wrap items-center gap-x-3 gap-y-2">
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
          :aria-label="s.label"
          :aria-pressed="shape === s.value"
          :disabled="refused(s) !== ''"
          :title="refused(s) || `Dessiner ${s.label.toLowerCase()} — crée la ligne qui la portera`"
          @click="emit('pick', s.value)"
          iconOnly
        />
      </ButtonGroup>
    </div>

    <Divider />

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
          aria-label="Trou"
          :aria-pressed="cuttingHole"
          :disabled="!hasAreal"
          :title="
            hasAreal
              ? 'Découper un trou dans le polygone qui le contient'
              : 'Aucun polygone dans la session : survolez-en un pour le rendre modifiable'
          "
          @click="emit('toggleHole')"
          iconOnly
        />
        <BaseButton
          joined
          size="sm"
          :variant="moving ? 'primary' : 'secondary'"
          :icon="Move"
          aria-label="Déplacer"
          :aria-pressed="moving"
          title="Déplacer la géométrie entière survolée, au lieu d’un seul de ses sommets"
          @click="emit('toggleMoving')"
          iconOnly
        />
        <BaseButton
          joined
          size="sm"
          :icon="Undo2"
          aria-label="Annuler le point"
          :disabled="!drawing"
          :title="drawing ? 'Retirer le dernier point posé' : 'Aucune forme en cours de dessin'"
          @click="emit('undo')"
          iconOnly
        />
      </ButtonGroup>
    </div>

    <Divider />

    <div class="flex items-center gap-1.5">
      <Typography variant="eyebrow">Aides</Typography>
      <ButtonGroup>
        <BaseButton
          joined
          size="sm"
          :variant="snapping ? 'primary' : 'secondary'"
          :icon="Magnet"
          aria-label="Aimantation"
          :aria-pressed="snapping"
          title="Aimanter un sommet posé sur les sommets et segments voisins"
          @click="emit('toggleSnapping')"
          iconOnly
        />
        <BaseButton
          joined
          size="sm"
          :variant="topological ? 'primary' : 'secondary'"
          :icon="Share2"
          aria-label="Topologie"
          :aria-pressed="topological"
          :title="
            topological
              ? 'Déplacer un sommet partagé déplace toutes les géométries qui le portent'
              : 'Seule la géométrie sous le pointeur est déplacée, ses voisines gardent leur sommet'
          "
          @click="emit('toggleTopology')"
          iconOnly
        />
      </ButtonGroup>
    </div>

    <EditSessionActions
      class="ml-auto"
      :changed-count="changedCount"
      :created-count="createdCount"
      :row-count="rowCount"
      :saving="saving"
      @save="emit('save')"
      @cancel="emit('cancel')"
    />
  </div>
</template>
