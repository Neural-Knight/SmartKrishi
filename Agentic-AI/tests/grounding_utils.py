def add_citations_to_text(text: str, grounding_supports: list, grounding_chunks: list) -> str:
    """
    Add inline citations to text based on grounding metadata.
    
    Args:
        text: The original text
        grounding_supports: List of grounding support objects
        grounding_chunks: List of grounding chunk objects (sources)
        
    Returns:
        Text with inline citations added
    """
    if not grounding_supports or not grounding_chunks:
        return text
    
    # Sort supports by end_index in descending order to avoid shifting issues when inserting
    sorted_supports = sorted(grounding_supports, key=lambda s: s.get('segment', {}).get('end_index', 0), reverse=True)
    
    result_text = text
    
    for support in sorted_supports:
        segment = support.get('segment', {})
        end_index = segment.get('end_index', 0)
        chunk_indices = support.get('grounding_chunk_indices', [])
        
        if chunk_indices and end_index <= len(result_text):
            # Create citation string like [1](link1), [2](link2)
            citation_links = []
            for i in chunk_indices:
                if i < len(grounding_chunks):
                    chunk = grounding_chunks[i]
                    uri = chunk.get('uri', '')
                    title = chunk.get('title', f'Source {i+1}')
                    citation_links.append(f"[{i + 1}]({uri} \"{title}\")")
            
            if citation_links:
                citation_string = " " + ", ".join(citation_links)
                # Make sure we don't go beyond the text length
                if end_index <= len(result_text):
                    result_text = result_text[:end_index] + citation_string + result_text[end_index:]
    
    return result_text


def extract_grounding_info(response_data: dict) -> dict:
    """
    Extract and format grounding information from API response.
    
    Args:
        response_data: The response data from /ask endpoint with logs=True
        
    Returns:
        Formatted grounding information
    """
    grounding = response_data.get('grounding', {})
    
    if not grounding:
        return {"has_grounding": False}
    
    info: dict = {"has_grounding": True}
    
    # Web search queries
    if 'web_search_queries' in grounding:
        info['search_queries'] = grounding['web_search_queries']
        info['search_count'] = len(grounding['web_search_queries'])
    
    # Sources
    if 'grounding_chunks' in grounding:
        chunks = grounding['grounding_chunks']
        info['sources'] = chunks
        info['source_count'] = len(chunks)
        info['source_domains'] = []
        
        for chunk in chunks:
            uri = chunk.get('uri', '')
            try:
                from urllib.parse import urlparse
                domain = urlparse(uri).netloc
                if domain and domain not in info['source_domains']:
                    info['source_domains'].append(domain)
            except:
                pass
    
    # Citations
    if 'grounding_supports' in grounding:
        supports = grounding['grounding_supports']
        info['citations'] = supports
        info['citation_count'] = len(supports)
        
        # Calculate text coverage
        total_text_length = len(response_data.get('answer', ''))
        cited_chars = 0
        
        for support in supports:
            segment = support.get('segment', {})
            start = segment.get('start_index', 0)
            end = segment.get('end_index', 0)
            cited_chars += max(0, end - start)
        
        if total_text_length > 0:
            info['citation_coverage'] = round((cited_chars / total_text_length) * 100, 1)
        else:
            info['citation_coverage'] = 0
    
    return info


def create_citation_summary(grounding_info: dict) -> str:
    """
    Create a human-readable summary of grounding information.
    
    Args:
        grounding_info: Output from extract_grounding_info()
        
    Returns:
        Summary string
    """
    if not grounding_info.get('has_grounding'):
        return "No grounding information available."
    
    parts = []
    
    if 'search_count' in grounding_info:
        parts.append(f"{grounding_info['search_count']} web searches performed")
    
    if 'source_count' in grounding_info:
        parts.append(f"{grounding_info['source_count']} sources found")
    
    if 'source_domains' in grounding_info:
        domains = grounding_info['source_domains']
        if domains:
            if len(domains) <= 3:
                parts.append(f"from domains: {', '.join(domains)}")
            else:
                parts.append(f"from {len(domains)} domains including {', '.join(domains[:2])}")
    
    if 'citation_coverage' in grounding_info:
        coverage = grounding_info['citation_coverage']
        if coverage > 0:
            parts.append(f"{coverage}% of text cited")
    
    if parts:
        return "Grounding: " + ", ".join(parts) + "."
    else:
        return "Grounding information present but could not be parsed."


# Example usage
if __name__ == "__main__":
    # Example grounding data
    example_text = "The 2024 Olympics were held in Paris. Spain won the football tournament."
    example_supports = [
        {
            "segment": {"start_index": 0, "end_index": 41, "text": "The 2024 Olympics were held in Paris."},
            "grounding_chunk_indices": [0, 1]
        },
        {
            "segment": {"start_index": 42, "end_index": 78, "text": "Spain won the football tournament."},
            "grounding_chunk_indices": [2]
        }
    ]
    example_chunks = [
        {"uri": "https://olympics.com/paris2024", "title": "Paris 2024 Olympics"},
        {"uri": "https://bbc.com/olympics", "title": "BBC Olympics Coverage"},
        {"uri": "https://fifa.com/results", "title": "FIFA Tournament Results"}
    ]
    
    print("Original text:")
    print(example_text)
    print("\nWith citations:")
    print(add_citations_to_text(example_text, example_supports, example_chunks))
