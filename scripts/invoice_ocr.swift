import Foundation
import ImageIO
import Vision

guard CommandLine.arguments.count >= 2 else {
    fputs("usage: swift invoice_ocr.swift <image-path>\n", stderr)
    exit(1)
}

let imagePath = CommandLine.arguments[1]
let imageURL = URL(fileURLWithPath: imagePath)

guard let imageSource = CGImageSourceCreateWithURL(imageURL as CFURL, nil),
      let image = CGImageSourceCreateImageAtIndex(imageSource, 0, nil) else {
    fputs("failed to load image: \(imagePath)\n", stderr)
    exit(1)
}

let request = VNRecognizeTextRequest()
request.recognitionLevel = .accurate
request.usesLanguageCorrection = false
request.minimumTextHeight = 0.01

let handler = VNImageRequestHandler(cgImage: image, options: [:])

do {
    try handler.perform([request])
} catch {
    fputs("ocr failed: \(error.localizedDescription)\n", stderr)
    exit(1)
}

let observations = (request.results ?? []).sorted { left, right in
    let leftMidY = left.boundingBox.midY
    let rightMidY = right.boundingBox.midY
    if abs(leftMidY - rightMidY) > 0.015 {
        return leftMidY > rightMidY
    }
    return left.boundingBox.minX < right.boundingBox.minX
}

for observation in observations {
    guard let candidate = observation.topCandidates(1).first else {
        continue
    }

    let line = candidate.string.trimmingCharacters(in: .whitespacesAndNewlines)
    if !line.isEmpty {
        print(line)
    }
}
