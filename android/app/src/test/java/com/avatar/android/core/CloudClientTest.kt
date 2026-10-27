package com.avatar.android.core

import com.avatar.android.core.cloud.CloudClient
import com.avatar.android.core.cloud.CloudConnectionState
import kotlinx.coroutines.runBlocking
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class CloudClientTest {

    private lateinit var mockServer: MockWebServer
    private lateinit var client: CloudClient

    @Before
    fun setUp() {
        mockServer = MockWebServer()
        mockServer.start()
        client = CloudClient(
            serverUrl = mockServer.url("").toString().removeSuffix("/"),
            userId = "user_test",
            deviceId = "pixel_10_dev",
            clientType = "pixel_10"
        )
    }

    @After
    fun tearDown() {
        mockServer.shutdown()
    }

    @Test
    fun testAcquireToken_WithoutConsent_FailsImmediately() = runBlocking {
        val res = client.acquireSessionToken(cloudConsent = false)
        assertTrue(res.isFailure)
        assertEquals(CloudConnectionState.DISCONNECTED, client.state)
    }

    @Test
    fun testAcquireToken_WithConsent_Succeeds() = runBlocking {
        mockServer.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setBody("""
                    {
                        "livekit_url": "ws://localhost:7880",
                        "room_name": "room_test_123",
                        "token": "jwt_token_sample",
                        "expires_in_seconds": 300
                    }
                """.trimIndent())
        )

        val res = client.acquireSessionToken(cloudConsent = true)
        assertTrue(res.isSuccess)
        assertEquals("room_test_123", client.currentRoom)
        assertEquals("jwt_token_sample", client.currentToken)
        assertEquals(CloudConnectionState.CONNECTED, client.state)

        // Audio gate must remain muted by default
        assertTrue(client.isAudioMuted)
    }

    @Test
    fun testAudioGate_OpensOnlyDuringActiveTurn() = runBlocking {
        mockServer.enqueue(
            MockResponse().setResponseCode(200).setBody("""
                {
                    "livekit_url": "ws://localhost:7880",
                    "room_name": "room_gate",
                    "token": "tok",
                    "expires_in_seconds": 300
                }
            """.trimIndent())
        )
        client.acquireSessionToken(cloudConsent = true)

        // Default: muted
        assertTrue(client.isAudioMuted)

        // Turn starts: media gate opens
        val started = client.startTurn("turn_1")
        assertTrue(started)
        assertFalse(client.isAudioMuted)

        // Turn finishes: media gate re-mutes
        client.finishTurn("turn_1")
        assertTrue(client.isAudioMuted)
    }

    @Test
    fun testInterruptActiveTurn_InstantlyMutesAudioAndNotifiesServer() = runBlocking {
        mockServer.enqueue(
            MockResponse().setResponseCode(200).setBody("""
                {
                    "livekit_url": "ws://localhost:7880",
                    "room_name": "room_interrupt",
                    "token": "tok",
                    "expires_in_seconds": 300
                }
            """.trimIndent())
        )
        client.acquireSessionToken(cloudConsent = true)
        client.startTurn("turn_active")
        assertFalse(client.isAudioMuted)

        mockServer.enqueue(MockResponse().setResponseCode(200).setBody("""{"status":"ok"}"""))

        val interruptRes = client.interruptActiveTurn("turn_active", "barge_in")
        assertTrue(interruptRes.isSuccess)
        // Audio must be muted instantly
        assertTrue(client.isAudioMuted)

        val recordedRequest = mockServer.takeRequest() // auth token request
        val interruptReq = mockServer.takeRequest() // interrupt request
        assertEquals("/api/v1/session/interrupt", interruptReq.path)
        val body = interruptReq.body.readUtf8()
        assertTrue(body.contains("room_interrupt"))
        assertTrue(body.contains("turn_active"))
    }
}
