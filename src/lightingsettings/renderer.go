package lightingsettings

import "LumenForge/src/rgb"

// RendererProfileFromEffectSettings converts one resolved canonical settings
// record into the legacy rgb.Profile input consumed by software renderers.
func RendererProfileFromEffectSettings(settings EffectSettings) rgb.Profile {
	profile := rgb.Profile{ProfileName: settings.EffectID, Brightness: 1}
	if descriptor, found := rgb.SoftwareEffectDescriptorByID(settings.EffectID); found {
		profile.Smoothness = descriptor.RendererSmoothness
	}
	if settings.Speed != nil {
		profile.Speed = *settings.Speed
	}
	if settings.SingleColor != nil {
		profile.StartColor = rendererColor(settings.SingleColor.Color)
	}
	if settings.TwoColor != nil {
		profile.StartColor = rendererColor(settings.TwoColor.Start)
		profile.EndColor = rendererColor(settings.TwoColor.End)
	}
	if settings.Temperature != nil {
		profile.StartColor = rendererTemperatureColor(settings.Temperature.Low)
		profile.MiddleColor = rendererTemperatureColor(settings.Temperature.Middle)
		profile.EndColor = rendererTemperatureColor(settings.Temperature.High)
		profile.MinTemp = settings.Temperature.Low.Celsius
		profile.MaxTemp = settings.Temperature.High.Celsius
	}
	if settings.Gradient != nil {
		profile.Gradients = make(map[int]rgb.Color, len(settings.Gradient.Stops))
		for index, stop := range settings.Gradient.Stops {
			color := rendererColor(stop.Color)
			color.Position = stop.Position
			color.Brightness = stop.Intensity
			profile.Gradients[index] = color
		}
	}
	return profile
}

func rendererColor(color Color) rgb.Color {
	return rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue, Brightness: 1}
}

func rendererTemperatureColor(point TemperaturePoint) rgb.Color {
	color := rendererColor(point.Color)
	color.Temperature = point.Celsius
	return color
}
