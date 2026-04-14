import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import { Link } from 'react-router-dom';
import L from 'leaflet';

type RestaurantPin = {
  id: string;
  name: string;
  city: string;
  address?: string;
  lat: number;
  lng: number;
};

// Починка путей к иконкам в сборках (Vite/ESM).
const markerIcon = new L.Icon({
  iconUrl: new URL('leaflet/dist/images/marker-icon.png', import.meta.url).toString(),
  iconRetinaUrl: new URL('leaflet/dist/images/marker-icon-2x.png', import.meta.url).toString(),
  shadowUrl: new URL('leaflet/dist/images/marker-shadow.png', import.meta.url).toString(),
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41],
});

export function RestaurantsMap({
  pins,
  center,
  zoom = 12,
}: {
  pins: RestaurantPin[];
  center: { lat: number; lng: number };
  zoom?: number;
}) {
  return (
    <div style={{ height: 420, borderRadius: 14, overflow: 'hidden', border: '1px solid var(--border)' }}>
      <MapContainer center={[center.lat, center.lng]} zoom={zoom} style={{ height: '100%', width: '100%' }}>
        <TileLayer
          attribution='&copy; OpenStreetMap contributors'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        {pins.map((p) => (
          <Marker key={p.id} position={[p.lat, p.lng]} icon={markerIcon}>
            <Popup>
              <div style={{ display: 'grid', gap: 6 }}>
                <strong>{p.name}</strong>
                <span className="muted compact">{p.address || p.city || '—'}</span>
                <Link to={`/restaurant/${p.id}`}>Открыть</Link>
              </div>
            </Popup>
          </Marker>
        ))}
      </MapContainer>
    </div>
  );
}

