#!/usr/bin/env python3
import sys
import os
import subprocess

def test_imports():
    """Test all required imports"""
    try:
        import groq
        import soundfile
        import pyaudio  
        import pynput
        import tempfile
        import subprocess
        
        print("✅ All Python packages imported successfully")
        print(f"✅ Groq package version: {groq.__version__}")
        
        return True
    except ImportError as e:
        print(f"❌ Import error: {e}")
        return False

def test_system_tools():
    """Test system tools availability"""
    display_server = os.environ.get('XDG_SESSION_TYPE', 'x11')
    
    tools_to_test = []
    if display_server == 'wayland':
        tools_to_test = ['wtype', 'ydotool', 'wl-copy']
    else:
        tools_to_test = ['xdotool', 'xclip']
    
    tools_to_test.extend(['pactl', 'notify-send'])
    
    success = True
    for tool in tools_to_test:
        try:
            subprocess.run([tool, '--help'], capture_output=True, timeout=5)
            print(f"✅ {tool} available")
        except (FileNotFoundError, subprocess.TimeoutExpired):
            print(f"❌ {tool} not found")
            success = False
    
    return success

def test_groq_api_key():
    """Test GROQ_API_KEY environment variable"""
    api_key = os.environ.get('GROQ_API_KEY')
    if api_key:
        print("✅ GROQ_API_KEY environment variable is set")
        return True
    else:
        print("⚠️  GROQ_API_KEY environment variable not set")
        print("   Set it with: export GROQ_API_KEY='your_api_key_here'")
        return False

if __name__ == "__main__":
    print("🧪 Testing Speech-to-Text Installation")
    print("=" * 40)
    
    tests = [
        ("Python packages", test_imports),
        ("System tools", test_system_tools), 
        ("Groq API key", test_groq_api_key)
    ]
    
    results = []
    for test_name, test_func in tests:
        print(f"\n🔍 Testing {test_name}...")
        results.append(test_func())
    
    print("\n" + "=" * 40)
    if all(results):
        print("🎉 All tests passed! Installation successful!")
        print("\nNext steps:")
        print("1. Run: ./run.sh")
    else:
        print("⚠️  Some tests failed. Check the output above.")
        if not os.environ.get('GROQ_API_KEY'):
            print("💡 Don't forget to set your GROQ_API_KEY!")
        sys.exit(1)
