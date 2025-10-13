# Midjourney Prompts for Broadcast Relay Visuals

## Icon Prompts

### Option 1: Modern Tech Icon
```
/imagine a minimalist logo icon for a Nostr broadcast relay service, purple gradient (#667eea to #764ba2), featuring interconnected broadcast waves radiating from a central node, modern tech aesthetic, clean geometric shapes, circular frame, white background, vector style, professional, 1024x1024 --v 6 --style raw
```

### Option 2: Signal Broadcast Icon  
```
/imagine a sleek relay icon showing signal waves broadcasting in all directions, purple and blue color scheme (#667eea, #2196F3), minimalist geometric design, circular badge format, tech startup aesthetic, gradient mesh, white background, 1024x1024 --v 6 --style raw
```

### Option 3: Network Hub Icon
```
/imagine a modern network hub icon with nodes and connections, representing a Nostr relay broadcaster, purple gradient theme (#667eea to #764ba2), isometric perspective, glowing connections, circular frame, white background, clean vector art, 1024x1024 --v 6 --style raw
```

### Option 4: Satellite Dish Tech
```
/imagine a stylized satellite dish icon broadcasting data streams, purple and violet gradients (#667eea, #764ba2), modern minimalist design, circular logo format, tech aesthetic, geometric shapes, white background, vector style, 1024x1024 --v 6 --style raw
```

### Option 5: Network Hub + Satellite Dish (Hybrid)
```
/imagine a modern network hub icon with satellite dish broadcasting signals to connected nodes, purple gradient (#667eea to #764ba2), isometric perspective with radiating broadcast waves, glowing interconnected nodes, circular frame, tech aesthetic, clean geometric shapes, white background, vector art, professional logo design, 1024x1024 --v 6 --style raw
```

## Banner Prompts

### Option 1: Central Broadcaster to Multiple Relays
```
/imagine a banner showing a central glowing hub broadcasting messages to multiple relay towers on the horizon, purple gradient sky (#667eea to #764ba2), stylized relay servers connected by light beams, modern illustrative style, clear visual narrative of message distribution, professional tech illustration --ar 3:1 --v 6 --style raw
```

### Option 2: Message Amplification Network
```
/imagine a banner illustrating a single message being amplified through a network of relay nodes, visual showing one input splitting into many outputs, purple and blue gradient (#667eea to #764ba2), clear data flow arrows, relay towers receiving signals, professional infographic style, modern and clean --ar 3:1 --v 6 --style raw
```

### Option 3: Global Relay Distribution
```
/imagine a banner with a stylized world map showing relay distribution, glowing connection lines from central point to multiple global relay locations, purple gradient theme (#667eea to #764ba2), showing broadcast reaching different continents, modern geographic visualization, professional and meaningful --ar 3:1 --v 6 --style raw
```

### Option 4: Event Broadcasting Pipeline
```
/imagine a banner showing the journey of a Nostr event through a broadcast relay system, left to right flow from single source to multiple destinations, relay towers and servers illustrated, purple gradient (#667eea to #764ba2), clear visual storytelling, modern technical illustration, professional design --ar 3:1 --v 6 --style raw
```

### Option 5: Connected Relay Ecosystem
```
/imagine a banner depicting interconnected Nostr relay servers forming an ecosystem, central broadcast relay distributing to surrounding relay nodes, purple gradient background (#667eea to #764ba2), showing collaborative network effect, servers and connection paths clearly visible, modern technical illustration --ar 3:1 --v 6 --style raw
```

## Recommended Specifications

### Icon
- **Size**: 1024x1024px (square)
- **Format**: PNG with transparency, or JPEG
- **Style**: Minimalist, modern, professional
- **Colors**: Purple gradient (#667eea to #764ba2) with white/blue accents
- **Usage**: Appears in relay info, NIP-11, and main page

### Banner
- **Size**: 1500x500px (3:1 ratio)
- **Format**: JPEG or PNG
- **Style**: Modern, abstract, tech-inspired
- **Colors**: Purple gradient theme matching icon
- **Usage**: Top of main page, creates professional impression

## Post-Processing Tips

1. **Icon**: 
   - Ensure good contrast on both light and dark backgrounds
   - Add subtle glow or shadow for depth
   - Keep it simple - should be recognizable even at 32x32px

2. **Banner**:
   - Test at different screen sizes (mobile, desktop)
   - Ensure text remains readable if overlaid
   - Use gradient overlay for better text contrast
   - Keep important elements center-focused

## Alternative: Use DALL-E or Stable Diffusion

These prompts can be adapted for other AI image generators:
- Remove `--v 6 --style raw` flags
- Keep the core description
- Adjust color codes as hex values
- Emphasize "professional" and "clean" for business use

## Where to Host

After generating images:
- Upload to IPFS (decentralized, permanent)
- Use imgbb.com or similar (free hosting)
- Host on your own server
- Use GitHub Pages or CDN

Update `.env` file with URLs:
```
RELAY_ICON=https://your-domain.com/icon.png
RELAY_BANNER=https://your-domain.com/banner.jpg
```

